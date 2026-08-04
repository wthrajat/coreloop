package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"coreloop/backend/internal/content"
)

type providerRoundTripFunc func(*http.Request) (*http.Response, error)

func (function providerRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestGroqRequestFitsTheFreeTierTPMLimit(t *testing.T) {
	var requestBody []byte
	client := &http.Client{Transport: providerRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		requestBody, _ = io.ReadAll(request.Body)
		return jsonResponse(request, `{"id":"request-1","choices":[{"message":{"content":"{}"}}]}`), nil
	})}

	provider := NewGroq("secret", "openai/gpt-oss-20b", client)
	if _, err := provider.Generate(context.Background(), "system", "input", content.JSONSchema(), 8_000); err != nil {
		t.Fatal(err)
	}

	var payload struct {
		MaxCompletionTokens int    `json:"max_completion_tokens"`
		ReasoningEffort     string `json:"reasoning_effort"`
		ResponseFormat      struct {
			Type string `json:"type"`
		} `json:"response_format"`
		Messages []struct {
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(requestBody, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.MaxCompletionTokens != 6_500 {
		t.Fatalf("Groq completion budget = %d, want 6500", payload.MaxCompletionTokens)
	}
	if payload.ReasoningEffort != "low" {
		t.Fatalf("Groq reasoning effort = %q, want low", payload.ReasoningEffort)
	}
	if payload.ResponseFormat.Type != "json_object" {
		t.Fatalf("Groq response format = %q, want json_object", payload.ResponseFormat.Type)
	}
	if len(payload.Messages) != 2 ||
		!strings.Contains(payload.Messages[0].Content, "every required property") ||
		!strings.Contains(payload.Messages[0].Content, `"production_example"`) ||
		!strings.Contains(payload.Messages[0].Content, `"uncertainty"`) {
		t.Fatalf("Groq system contract was not included: %#v", payload.Messages)
	}
	promptTokens := content.EstimateTokens(payload.Messages[0].Content) +
		content.EstimateTokens(payload.Messages[1].Content)
	if promptTokens+payload.MaxCompletionTokens+groqTokenBudgetReserve > groqFreeTierTokenBudget {
		t.Fatalf("Groq request budget = %d, exceeds %d", promptTokens+payload.MaxCompletionTokens+groqTokenBudgetReserve, groqFreeTierTokenBudget)
	}
}

func TestGroqThirtyMinutePromptKeepsEnoughDetailedOutputBudget(t *testing.T) {
	var requestBody []byte
	client := &http.Client{Transport: providerRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		requestBody, _ = io.ReadAll(request.Body)
		return jsonResponse(request, `{"id":"request-1","choices":[{"message":{"content":"{}"}}]}`), nil
	})}
	system, input, err := content.Compile(content.LessonContext{
		Topic: "Database replication", Minutes: 30, Depth: "detailed",
		Objectives: []string{"Explain replication mechanics and production failure recovery"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	provider := NewGroq("secret", "openai/gpt-oss-20b", client)
	if _, err := provider.Generate(
		context.Background(), system, input, content.JSONSchema(), content.OutputBudget(30),
	); err != nil {
		t.Fatal(err)
	}

	var payload struct {
		MaxCompletionTokens int `json:"max_completion_tokens"`
		Messages            []struct {
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(requestBody, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.MaxCompletionTokens < 5_000 {
		t.Fatalf("30-minute Groq completion budget = %d, want at least 5000", payload.MaxCompletionTokens)
	}
	promptTokens := 0
	for _, message := range payload.Messages {
		promptTokens += content.EstimateTokens(message.Content)
	}
	if total := promptTokens + payload.MaxCompletionTokens + groqTokenBudgetReserve; total > groqFreeTierTokenBudget {
		t.Fatalf("30-minute Groq request budget = %d, exceeds %d", total, groqFreeTierTokenBudget)
	}
}

func TestOpenAIKeepsStrictStructuredOutput(t *testing.T) {
	var requestBody []byte
	client := &http.Client{Transport: providerRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		requestBody, _ = io.ReadAll(request.Body)
		return jsonResponse(request, `{"id":"request-1","choices":[{"message":{"content":"{}"}}]}`), nil
	})}

	provider := NewOpenAI("secret", "gpt-test", client)
	if _, err := provider.Generate(context.Background(), "system", "input", map[string]any{"type": "object"}, 4_500); err != nil {
		t.Fatal(err)
	}

	var payload struct {
		ResponseFormat struct {
			Type       string `json:"type"`
			JSONSchema struct {
				Strict bool `json:"strict"`
			} `json:"json_schema"`
		} `json:"response_format"`
	}
	if err := json.Unmarshal(requestBody, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.ResponseFormat.Type != "json_schema" || !payload.ResponseFormat.JSONSchema.Strict {
		t.Fatalf("OpenAI response format = %#v", payload.ResponseFormat)
	}
}

func TestGeminiRequestUsesTheStructuredOutputMIMEEnum(t *testing.T) {
	var requestBody []byte
	client := &http.Client{Transport: providerRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		requestBody, _ = io.ReadAll(request.Body)
		return jsonResponse(request, `{"responseId":"request-1","candidates":[{"content":{"parts":[{"text":"{}"}]}}]}`), nil
	})}

	provider := NewGemini("secret", "gemini-3.6-flash", client)
	if _, err := provider.Generate(context.Background(), "system", "input", map[string]any{"type": "object"}, 4_500); err != nil {
		t.Fatal(err)
	}

	var payload struct {
		GenerationConfig struct {
			ResponseFormat struct {
				Text struct {
					MIMEType string `json:"mimeType"`
				} `json:"text"`
			} `json:"responseFormat"`
		} `json:"generationConfig"`
	}
	if err := json.Unmarshal(requestBody, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.GenerationConfig.ResponseFormat.Text.MIMEType != "APPLICATION_JSON" {
		t.Fatalf("Gemini structured-output MIME type = %q, want APPLICATION_JSON", payload.GenerationConfig.ResponseFormat.Text.MIMEType)
	}
}

func jsonResponse(request *http.Request, body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
		Request:    request,
	}
}
