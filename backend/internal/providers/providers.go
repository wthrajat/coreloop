package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"coreloop/backend/internal/content"
)

type FailureKind string

const (
	FailureQuota     FailureKind = "quota_exhausted"
	FailureTransient FailureKind = "transient"
	FailurePermanent FailureKind = "permanent"
	FailureInvalid   FailureKind = "invalid_output"
)

type Error struct {
	Provider string
	Kind     FailureKind
	Status   int
	Message  string
	Cause    error
}

func (err *Error) Error() string {
	return fmt.Sprintf("%s %s: %s", err.Provider, err.Kind, err.Message)
}
func (err *Error) Unwrap() error { return err.Cause }

type Response struct {
	Body         []byte
	RequestID    string
	InputTokens  int
	OutputTokens int
	CachedTokens int
}

type Provider interface {
	Name() string
	Model() string
	Configured() bool
	Generate(context.Context, string, string, map[string]any, int) (Response, error)
}

type Router struct {
	free                    []Provider
	openAI                  Provider
	now                     func() time.Time
	freeAttemptTimeout      time.Duration
	finalFreeAttemptTimeout time.Duration
}

const (
	defaultFreeProviderAttemptTimeout      = 18 * time.Second
	defaultFinalFreeProviderAttemptTimeout = 32 * time.Second
	providerFinalizationReserve            = 6 * time.Second
)

func NewRouter(groq, gemini, openAI Provider) *Router {
	var free []Provider
	if groq != nil {
		free = append(free, groq)
	}
	if gemini != nil {
		free = append(free, gemini)
	}
	return &Router{
		free: free, openAI: openAI, now: time.Now,
		freeAttemptTimeout:      defaultFreeProviderAttemptTimeout,
		finalFreeAttemptTimeout: defaultFinalFreeProviderAttemptTimeout,
	}
}

var ErrFreeQuotaExhausted = errors.New("all configured free AI providers are unavailable or out of quota")

func (router *Router) Generate(ctx context.Context, lessonContext content.LessonContext) (content.Generated, error) {
	var failures []error
	configured := make([]Provider, 0, len(router.free))
	for _, provider := range router.free {
		if !provider.Configured() {
			continue
		}
		configured = append(configured, provider)
	}
	for index, provider := range configured {
		attemptTimeout := router.providerAttemptTimeout(ctx, index == len(configured)-1)
		started := router.now()
		slog.InfoContext(
			ctx,
			"AI provider attempt started",
			"provider", provider.Name(),
			"model", provider.Model(),
			"timeout_ms", attemptTimeout.Milliseconds(),
		)
		attemptContext, cancelAttempt := context.WithTimeout(ctx, attemptTimeout)
		generated, err := generateWithCorrection(attemptContext, provider, lessonContext)
		cancelAttempt()
		if err == nil {
			slog.InfoContext(
				ctx,
				"AI provider attempt succeeded",
				"provider", provider.Name(),
				"model", provider.Model(),
				"duration_ms", router.now().Sub(started).Milliseconds(),
			)
			return generated, nil
		}
		var providerError *Error
		errors.As(err, &providerError)
		failureKind := FailureTransient
		status := 0
		if providerError != nil {
			failureKind = providerError.Kind
			status = providerError.Status
		}
		slog.WarnContext(
			ctx,
			"AI provider attempt failed",
			"provider", provider.Name(),
			"model", provider.Model(),
			"failure_kind", failureKind,
			"http_status", status,
			"duration_ms", router.now().Sub(started).Milliseconds(),
			"error", err,
		)
		failures = append(failures, err)
	}
	if len(failures) == 0 {
		return content.Generated{}, errors.New("no free AI provider is configured")
	}
	joined := errors.Join(failures...)
	if everyFailureIsQuotaExhaustion(failures) {
		return content.Generated{}, fmt.Errorf("%w: %v", ErrFreeQuotaExhausted, joined)
	}
	return content.Generated{}, fmt.Errorf("all configured free AI providers failed: %w", joined)
}

func (router *Router) providerAttemptTimeout(ctx context.Context, finalProvider bool) time.Duration {
	timeout := router.freeAttemptTimeout
	maximum := defaultFreeProviderAttemptTimeout
	if finalProvider {
		timeout = router.finalFreeAttemptTimeout
		maximum = defaultFinalFreeProviderAttemptTimeout
	}
	if timeout <= 0 || timeout > maximum {
		timeout = maximum
	}
	if deadline, ok := ctx.Deadline(); ok {
		available := time.Until(deadline) - providerFinalizationReserve
		if available < timeout {
			timeout = available
		}
	}
	if timeout <= 0 {
		return time.Nanosecond
	}
	return timeout
}

func everyFailureIsQuotaExhaustion(failures []error) bool {
	for _, failure := range failures {
		var providerError *Error
		if !errors.As(failure, &providerError) || providerError.Kind != FailureQuota {
			return false
		}
	}
	return len(failures) > 0
}

func (router *Router) GenerateWithOpenAI(ctx context.Context, lessonContext content.LessonContext) (content.Generated, error) {
	if router.openAI == nil || !router.openAI.Configured() {
		return content.Generated{}, errors.New("OpenAI is not configured")
	}
	return generateWithCorrection(ctx, router.openAI, lessonContext)
}

func generateWithCorrection(ctx context.Context, provider Provider, lessonContext content.LessonContext) (content.Generated, error) {
	var correction []string
	var last content.Generated
	var best content.Generated
	for attempt := 0; attempt < 2; attempt++ {
		system, input, err := content.Compile(lessonContext, correction)
		if err != nil {
			if usableWithCorrectionWarning(&best) {
				return best, nil
			}
			return content.Generated{}, err
		}
		response, err := provider.Generate(ctx, system, input, content.JSONSchema(), content.OutputBudget(lessonContext.Minutes))
		if err != nil {
			if usableWithCorrectionWarning(&best) {
				return best, nil
			}
			return content.Generated{}, err
		}
		var draft content.LessonDraft
		if err := json.Unmarshal(response.Body, &draft); err != nil {
			correction = []string{"Return one valid JSON object matching the provided schema; the prior response could not be parsed."}
			last = content.Generated{Provider: provider.Name(), Model: provider.Model(), RequestID: response.RequestID,
				InputTokens: response.InputTokens, OutputTokens: response.OutputTokens, ValidationErrors: correction}
			continue
		}
		problems, verification := content.Validate(draft, lessonContext.Minutes, lessonContext.Evidence)
		last = content.Generated{Draft: draft, Provider: provider.Name(), Model: provider.Model(), RequestID: response.RequestID,
			InputTokens: response.InputTokens, OutputTokens: response.OutputTokens, ValidationErrors: problems, VerificationState: verification}
		if content.Usable(draft) {
			best = last
		}
		if len(problems) == 0 {
			return last, nil
		}
		if attempt == 0 {
			correction = problems
			continue
		}
		if content.Usable(draft) {
			usableWithCorrectionWarning(&last)
			return last, nil
		}
	}
	if usableWithCorrectionWarning(&best) {
		return best, nil
	}
	return content.Generated{}, &Error{Provider: provider.Name(), Kind: FailureInvalid, Message: "two responses were unusable"}
}

func usableWithCorrectionWarning(generated *content.Generated) bool {
	if !content.Usable(generated.Draft) {
		return false
	}
	generated.Warning = "The lesson did not fully pass structural or source verification after one correction attempt. The information is delivered with this warning so you can judge it directly."
	generated.VerificationState = "unverified_warning"
	return true
}
