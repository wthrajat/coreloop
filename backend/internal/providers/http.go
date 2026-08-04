package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type OpenAICompatible struct {
	provider string
	endpoint string
	apiKey   string
	model    string
	http     *http.Client
}

const groqFreeTierCompletionBudget = 6_500

func NewGroq(apiKey, model string, client *http.Client) *OpenAICompatible {
	return newOpenAICompatible("groq", "https://api.groq.com/openai/v1/chat/completions", apiKey, model, client)
}

func NewOpenAI(apiKey, model string, client *http.Client) *OpenAICompatible {
	return newOpenAICompatible("openai", "https://api.openai.com/v1/chat/completions", apiKey, model, client)
}

func newOpenAICompatible(provider, endpoint, apiKey, model string, client *http.Client) *OpenAICompatible {
	if client == nil {
		client = &http.Client{Timeout: 50 * time.Second}
	}
	return &OpenAICompatible{provider: provider, endpoint: endpoint, apiKey: apiKey, model: model, http: client}
}

func (provider *OpenAICompatible) Name() string  { return provider.provider }
func (provider *OpenAICompatible) Model() string { return provider.model }
func (provider *OpenAICompatible) Configured() bool {
	return provider.apiKey != "" && provider.model != ""
}

func (provider *OpenAICompatible) Generate(ctx context.Context, system, input string, schema map[string]any, outputBudget int) (Response, error) {
	payload := map[string]any{
		"model":    provider.model,
		"messages": []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": input}},
		"response_format": map[string]any{"type": "json_schema", "json_schema": map[string]any{
			"name": "lesson_draft", "strict": true, "schema": schema,
		}},
		"max_completion_tokens": provider.completionBudget(outputBudget),
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return Response{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, provider.endpoint, bytes.NewReader(encoded))
	if err != nil {
		return Response{}, err
	}
	request.Header.Set("Authorization", "Bearer "+provider.apiKey)
	request.Header.Set("Content-Type", "application/json")
	response, err := provider.http.Do(request)
	if err != nil {
		return Response{}, &Error{Provider: provider.provider, Kind: FailureTransient, Message: "request failed", Cause: err}
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return Response{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return Response{}, classifyHTTP(provider.provider, response.StatusCode, body)
	}
	var decoded struct {
		ID      string `json:"id"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
				Refusal string `json:"refusal"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			PromptDetails    struct {
				CachedTokens int `json:"cached_tokens"`
			} `json:"prompt_tokens_details"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return Response{}, &Error{Provider: provider.provider, Kind: FailureInvalid, Message: "invalid response envelope", Cause: err}
	}
	if len(decoded.Choices) == 0 || decoded.Choices[0].Message.Content == "" {
		message := "response contained no content"
		if len(decoded.Choices) > 0 && decoded.Choices[0].Message.Refusal != "" {
			message = decoded.Choices[0].Message.Refusal
		}
		return Response{}, &Error{Provider: provider.provider, Kind: FailureInvalid, Message: message}
	}
	return Response{Body: []byte(decoded.Choices[0].Message.Content), RequestID: decoded.ID,
		InputTokens: decoded.Usage.PromptTokens, OutputTokens: decoded.Usage.CompletionTokens,
		CachedTokens: decoded.Usage.PromptDetails.CachedTokens}, nil
}

func (provider *OpenAICompatible) completionBudget(requested int) int {
	if provider.provider == "groq" && requested > groqFreeTierCompletionBudget {
		return groqFreeTierCompletionBudget
	}
	return requested
}

type Gemini struct {
	apiKey, model string
	http          *http.Client
}

func NewGemini(apiKey, model string, client *http.Client) *Gemini {
	if client == nil {
		client = &http.Client{Timeout: 50 * time.Second}
	}
	return &Gemini{apiKey: apiKey, model: model, http: client}
}
func (*Gemini) Name() string              { return "gemini" }
func (provider *Gemini) Model() string    { return provider.model }
func (provider *Gemini) Configured() bool { return provider.apiKey != "" && provider.model != "" }

func (provider *Gemini) Generate(ctx context.Context, system, input string, schema map[string]any, outputBudget int) (Response, error) {
	endpoint := "https://generativelanguage.googleapis.com/v1beta/models/" + provider.model + ":generateContent"
	payload := map[string]any{
		"system_instruction": map[string]any{"parts": []map[string]string{{"text": system}}},
		"contents":           []map[string]any{{"role": "user", "parts": []map[string]string{{"text": input}}}},
		"generationConfig": map[string]any{
			"maxOutputTokens": outputBudget, "temperature": 0.2,
			"responseFormat": map[string]any{"text": map[string]any{"mimeType": "APPLICATION_JSON", "schema": schema}},
		},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return Response{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return Response{}, err
	}
	request.Header.Set("x-goog-api-key", provider.apiKey)
	request.Header.Set("Content-Type", "application/json")
	response, err := provider.http.Do(request)
	if err != nil {
		return Response{}, &Error{Provider: "gemini", Kind: FailureTransient, Message: "request failed", Cause: err}
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return Response{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return Response{}, classifyHTTP("gemini", response.StatusCode, body)
	}
	var decoded struct {
		ResponseID string `json:"responseId"`
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		Usage struct {
			Prompt     int `json:"promptTokenCount"`
			Candidates int `json:"candidatesTokenCount"`
			Cached     int `json:"cachedContentTokenCount"`
		} `json:"usageMetadata"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return Response{}, &Error{Provider: "gemini", Kind: FailureInvalid, Message: "invalid response envelope", Cause: err}
	}
	var output strings.Builder
	if len(decoded.Candidates) > 0 {
		for _, part := range decoded.Candidates[0].Content.Parts {
			output.WriteString(part.Text)
		}
	}
	if output.Len() == 0 {
		return Response{}, &Error{Provider: "gemini", Kind: FailureInvalid, Message: "response contained no content"}
	}
	return Response{Body: []byte(output.String()), RequestID: decoded.ResponseID, InputTokens: decoded.Usage.Prompt,
		OutputTokens: decoded.Usage.Candidates, CachedTokens: decoded.Usage.Cached}, nil
}

func classifyHTTP(provider string, status int, body []byte) error {
	message := strings.TrimSpace(string(body))
	if len(message) > 500 {
		message = message[:500]
	}
	kind := FailurePermanent
	if status == http.StatusTooManyRequests || status == http.StatusPaymentRequired || strings.Contains(strings.ToLower(message), "quota") {
		kind = FailureQuota
	} else if status >= 500 || status == http.StatusRequestTimeout {
		kind = FailureTransient
	}
	return &Error{Provider: provider, Kind: kind, Status: status, Message: fmt.Sprintf("HTTP %d: %s", status, message)}
}
