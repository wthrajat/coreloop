package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"coreloop/backend/internal/content"
	"coreloop/backend/internal/providers"
	"coreloop/backend/internal/telegram"
)

func TestGeneratedLessonReachesTelegramContract(t *testing.T) {
	draft := deliveryContractDraft()
	encodedDraft, err := json.Marshal(draft)
	if err != nil {
		t.Fatal(err)
	}
	encodedEnvelope, err := json.Marshal(map[string]any{
		"id": "groq-request", "choices": []any{map[string]any{
			"message": map[string]string{"content": string(encodedDraft)},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	providerClient := &http.Client{Transport: jobRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(encodedEnvelope)),
			Header:     make(http.Header),
			Request:    request,
		}, nil
	})}
	router := providers.NewRouter(
		providers.NewGroq("test-key", "openai/gpt-oss-20b", providerClient),
		nil,
		nil,
	)
	generated, err := router.Generate(context.Background(), content.LessonContext{Minutes: 15})
	if err != nil {
		t.Fatalf("generate lesson: %v", err)
	}

	parts := telegram.ChunkHTML(content.RenderSections(generated.Draft), generated.Warning)
	if len(parts) < 2 {
		t.Fatalf("detailed lesson was not split for Telegram: %d part(s)", len(parts))
	}
	sentParts := 0
	telegramClient := telegram.New("test-token", &http.Client{Transport: jobRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/bottest-token/sendMessage" {
			t.Fatalf("Telegram path = %q", request.URL.Path)
		}
		var payload struct {
			ChatID string `json:"chat_id"`
			Text   string `json:"text"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.ChatID != "123456789" || strings.TrimSpace(payload.Text) == "" {
			t.Fatalf("Telegram payload = %#v", payload)
		}
		sentParts++
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"ok":true,"result":{"message_id":42}}`)),
			Header:     make(http.Header),
			Request:    request,
		}, nil
	})})
	for index, part := range parts {
		options := telegram.MessageOptions{}
		if index == len(parts)-1 {
			options.Buttons = [][]telegram.Button{{
				{Text: "Read", Data: "read:assignment-test"},
				{Text: "Skip", Data: "skip:assignment-test"},
			}}
		}
		if _, err := telegramClient.SendMessage(context.Background(), "123456789", part, options); err != nil {
			t.Fatalf("send Telegram part %d: %v", index+1, err)
		}
	}
	if sentParts != len(parts) {
		t.Fatalf("sent Telegram parts = %d, want %d", sentParts, len(parts))
	}
}

func deliveryContractDraft() content.LessonDraft {
	detail := strings.Repeat(
		"precise engineering detail with production tradeoffs and failure analysis ",
		12,
	)
	return content.LessonDraft{
		Title: "A production lesson", EstimatedMinutes: 15, Motivation: detail,
		PriorApproaches: []string{detail}, Definition: detail, Mechanics: []string{detail},
		ProductionExample: detail, Tradeoffs: []string{detail}, FailureModes: []string{detail},
		WhenNotToUse: []string{detail}, Alternatives: []string{detail}, Security: detail,
		Reliability: detail, Performance: detail, Cost: detail, PresentMaturity: detail,
		FutureDirection: detail, CareerRelevance: detail, InterviewAnswer: detail,
		RecallQuestion: "Which tradeoff would you evaluate first?",
		Claims:         []content.Claim{}, Sources: []content.Source{}, Uncertainty: []string{},
	}
}
