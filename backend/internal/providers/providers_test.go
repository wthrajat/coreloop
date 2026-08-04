package providers

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"coreloop/backend/internal/content"
)

type fakeProvider struct {
	name       string
	configured bool
	responses  []any
	calls      int
}

type blockingProvider struct {
	name  string
	calls int
}

type delayedProvider struct {
	name     string
	delay    time.Duration
	response any
	calls    int
}

func (provider *blockingProvider) Name() string { return provider.name }
func (*blockingProvider) Model() string         { return "test" }
func (*blockingProvider) Configured() bool      { return true }
func (provider *blockingProvider) Generate(ctx context.Context, _ string, _ string, _ map[string]any, _ int) (Response, error) {
	provider.calls++
	<-ctx.Done()
	return Response{}, &Error{Provider: provider.name, Kind: FailureTransient, Message: "request timed out", Cause: ctx.Err()}
}

func (provider *delayedProvider) Name() string { return provider.name }
func (*delayedProvider) Model() string         { return "test" }
func (*delayedProvider) Configured() bool      { return true }
func (provider *delayedProvider) Generate(
	ctx context.Context,
	_ string,
	_ string,
	_ map[string]any,
	_ int,
) (Response, error) {
	provider.calls++
	timer := time.NewTimer(provider.delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return Response{}, &Error{
			Provider: provider.name,
			Kind:     FailureTransient,
			Message:  "request timed out",
			Cause:    ctx.Err(),
		}
	case <-timer.C:
	}
	encoded, _ := json.Marshal(provider.response)
	return Response{Body: encoded}, nil
}

func (provider *fakeProvider) Name() string     { return provider.name }
func (*fakeProvider) Model() string             { return "test" }
func (provider *fakeProvider) Configured() bool { return provider.configured }
func (provider *fakeProvider) Generate(context.Context, string, string, map[string]any, int) (Response, error) {
	value := provider.responses[provider.calls]
	provider.calls++
	if err, ok := value.(error); ok {
		return Response{}, err
	}
	encoded, _ := json.Marshal(value)
	return Response{Body: encoded}, nil
}

func TestRouterFallsBackWithoutUsingOpenAI(t *testing.T) {
	groq := &fakeProvider{name: "groq", configured: true, responses: []any{&Error{Provider: "groq", Kind: FailureQuota, Message: "quota"}}}
	gemini := &fakeProvider{name: "gemini", configured: true, responses: []any{validProviderDraft()}}
	openAI := &fakeProvider{name: "openai", configured: true, responses: []any{errors.New("must not run")}}
	generated, err := NewRouter(groq, gemini, openAI).Generate(context.Background(), content.LessonContext{Minutes: 15})
	if err != nil {
		t.Fatal(err)
	}
	if generated.Provider != "gemini" {
		t.Fatalf("provider=%s", generated.Provider)
	}
	if openAI.calls != 0 {
		t.Fatal("OpenAI was called automatically")
	}
}

func TestRouterPreservesTimeForGeminiWhenGroqHangs(t *testing.T) {
	groq := &blockingProvider{name: "groq"}
	gemini := &fakeProvider{name: "gemini", configured: true, responses: []any{validProviderDraft()}}
	router := NewRouter(groq, gemini, nil)
	router.freeAttemptTimeout = 20 * time.Millisecond

	started := time.Now()
	generated, err := router.Generate(context.Background(), content.LessonContext{Minutes: 15})
	if err != nil {
		t.Fatal(err)
	}
	if generated.Provider != "gemini" || groq.calls != 1 || gemini.calls != 1 {
		t.Fatalf("fallback result = %#v, Groq calls = %d, Gemini calls = %d", generated, groq.calls, gemini.calls)
	}
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("fallback took %s", elapsed)
	}
}

func TestRouterGivesTheFinalFallbackMoreTime(t *testing.T) {
	groq := &fakeProvider{name: "groq", configured: true, responses: []any{
		&Error{Provider: "groq", Kind: FailurePermanent, Message: "structured output rejected"},
	}}
	gemini := &delayedProvider{
		name: "gemini", delay: 40 * time.Millisecond, response: validProviderDraft(),
	}
	router := NewRouter(groq, gemini, nil)
	router.freeAttemptTimeout = 10 * time.Millisecond
	router.finalFreeAttemptTimeout = 100 * time.Millisecond

	generated, err := router.Generate(context.Background(), content.LessonContext{Minutes: 15})
	if err != nil {
		t.Fatal(err)
	}
	if generated.Provider != "gemini" || gemini.calls != 1 {
		t.Fatalf("fallback result = %#v, Gemini calls = %d", generated, gemini.calls)
	}
}

func TestRouterDeliversAUsableDraftWhenItsCorrectionRequestFails(t *testing.T) {
	partial := content.LessonDraft{
		Title: "A useful partial lesson", Motivation: "Why this matters", Definition: "A precise definition",
		Mechanics: []string{"The core mechanism"}, EstimatedMinutes: 15,
	}
	groq := &fakeProvider{name: "groq", configured: true, responses: []any{
		partial,
		&Error{Provider: "groq", Kind: FailureTransient, Message: "correction timed out"},
	}}

	generated, err := NewRouter(groq, nil, nil).Generate(
		context.Background(),
		content.LessonContext{Minutes: 15},
	)
	if err != nil {
		t.Fatal(err)
	}
	if groq.calls != 2 || generated.Draft.Title != partial.Title {
		t.Fatalf("generated = %#v, provider calls = %d", generated, groq.calls)
	}
	if generated.VerificationState != "unverified_warning" || generated.Warning == "" {
		t.Fatalf("usable correction fallback was not marked with a warning: %#v", generated)
	}
}

func TestRouterDoesNotReportQuotaForProviderContractFailures(t *testing.T) {
	groq := &fakeProvider{name: "groq", configured: true, responses: []any{
		&Error{Provider: "groq", Kind: FailurePermanent, Message: "request exceeds the model TPM limit"},
	}}
	gemini := &fakeProvider{name: "gemini", configured: true, responses: []any{
		&Error{Provider: "gemini", Kind: FailurePermanent, Message: "invalid structured-output request"},
	}}

	_, err := NewRouter(groq, gemini, nil).Generate(context.Background(), content.LessonContext{Minutes: 30})
	if err == nil {
		t.Fatal("expected provider failure")
	}
	if errors.Is(err, ErrFreeQuotaExhausted) {
		t.Fatalf("provider contract errors were mislabeled as quota exhaustion: %v", err)
	}
}

func TestRouterReportsQuotaOnlyWhenEveryConfiguredFreeProviderIsExhausted(t *testing.T) {
	groq := &fakeProvider{name: "groq", configured: true, responses: []any{
		&Error{Provider: "groq", Kind: FailureQuota, Message: "quota"},
	}}
	gemini := &fakeProvider{name: "gemini", configured: true, responses: []any{
		&Error{Provider: "gemini", Kind: FailureQuota, Message: "quota"},
	}}

	_, err := NewRouter(groq, gemini, nil).Generate(context.Background(), content.LessonContext{Minutes: 30})
	if !errors.Is(err, ErrFreeQuotaExhausted) {
		t.Fatalf("quota exhaustion error = %v", err)
	}
}

func validProviderDraft() content.LessonDraft {
	long := strings.Repeat("technical detail ", 180)
	return content.LessonDraft{Title: "T", EstimatedMinutes: 15, Motivation: long, PriorApproaches: []string{long}, Definition: long, Mechanics: []string{long}, ProductionExample: long, Tradeoffs: []string{long}, FailureModes: []string{long}, WhenNotToUse: []string{long}, Alternatives: []string{long}, Security: long, Reliability: long, Performance: long, Cost: long, PresentMaturity: long, FutureDirection: long, CareerRelevance: long, InterviewAnswer: long, RecallQuestion: "R"}
}
