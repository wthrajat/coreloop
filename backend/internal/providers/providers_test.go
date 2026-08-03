package providers

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"coreloop/backend/internal/content"
)

type fakeProvider struct {
	name       string
	configured bool
	responses  []any
	calls      int
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
	long := strings.Repeat("technical detail ", 180)
	draft := content.LessonDraft{Title: "T", EstimatedMinutes: 15, Motivation: long, PriorApproaches: []string{long}, Definition: long, Mechanics: []string{long}, ProductionExample: long, Tradeoffs: []string{long}, FailureModes: []string{long}, WhenNotToUse: []string{long}, Alternatives: []string{long}, Security: long, Reliability: long, Performance: long, Cost: long, PresentMaturity: long, FutureDirection: long, CareerRelevance: long, InterviewAnswer: long, RecallQuestion: "R"}
	gemini := &fakeProvider{name: "gemini", configured: true, responses: []any{draft}}
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
