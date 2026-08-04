package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	free   []Provider
	openAI Provider
	now    func() time.Time
}

func NewRouter(groq, gemini, openAI Provider) *Router {
	var free []Provider
	if groq != nil {
		free = append(free, groq)
	}
	if gemini != nil {
		free = append(free, gemini)
	}
	return &Router{free: free, openAI: openAI, now: time.Now}
}

var ErrFreeQuotaExhausted = errors.New("all configured free AI providers are unavailable or out of quota")

func (router *Router) Generate(ctx context.Context, lessonContext content.LessonContext) (content.Generated, error) {
	var failures []error
	for _, provider := range router.free {
		if !provider.Configured() {
			continue
		}
		generated, err := generateWithCorrection(ctx, provider, lessonContext)
		if err == nil {
			return generated, nil
		}
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
	for attempt := 0; attempt < 2; attempt++ {
		system, input, err := content.Compile(lessonContext, correction)
		if err != nil {
			return content.Generated{}, err
		}
		response, err := provider.Generate(ctx, system, input, content.JSONSchema(), content.OutputBudget(lessonContext.Minutes))
		if err != nil {
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
		if len(problems) == 0 {
			return last, nil
		}
		if attempt == 0 {
			correction = problems
			continue
		}
		if content.Usable(draft) {
			last.Warning = "The lesson did not fully pass structural or source verification after one correction. The information is delivered with this warning so you can judge it directly."
			last.VerificationState = "unverified_warning"
			return last, nil
		}
	}
	return content.Generated{}, &Error{Provider: provider.Name(), Kind: FailureInvalid, Message: "two responses were unusable"}
}
