package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
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
	if _, err := provider.Generate(context.Background(), "system", "input", map[string]any{"type": "object"}, 8_000); err != nil {
		t.Fatal(err)
	}

	var payload struct {
		MaxCompletionTokens int `json:"max_completion_tokens"`
	}
	if err := json.Unmarshal(requestBody, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.MaxCompletionTokens != 6_500 {
		t.Fatalf("Groq completion budget = %d, want 6500", payload.MaxCompletionTokens)
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
