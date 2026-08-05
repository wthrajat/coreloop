package jobs

import (
	"errors"
	"strings"
	"testing"

	"coreloop/backend/internal/providers"
	"coreloop/backend/internal/store"
	"coreloop/backend/internal/telegram"
)

func TestClassifyJobFailureDoesNotPersistProviderPayloads(t *testing.T) {
	secretPayload := "api-key=secret generated lesson body"
	failure := errors.Join(
		&providers.Error{
			Provider: "groq",
			Kind:     providers.FailureInvalid,
			Status:   400,
			Message:  secretPayload,
			Cause:    errors.New(secretPayload),
		},
		&providers.Error{
			Provider: "gemini",
			Kind:     providers.FailureTransient,
			Status:   503,
			Message:  secretPayload,
		},
	)

	code, summary := classifyJobFailure(
		store.Job{Type: "generate_lesson"},
		failure,
		false,
	)
	if code != "ai_invalid_output" {
		t.Fatalf("code = %q", code)
	}
	for _, expected := range []string{
		"Groq returned invalid lesson output (HTTP 400)",
		"Gemini had a temporary provider failure (HTTP 503)",
	} {
		if !strings.Contains(summary, expected) {
			t.Fatalf("summary %q is missing %q", summary, expected)
		}
	}
	if strings.Contains(summary, secretPayload) || strings.Contains(summary, "secret") {
		t.Fatalf("provider payload leaked into diagnostic: %q", summary)
	}
}

func TestClassifyJobFailureDoesNotPersistTelegramDescription(t *testing.T) {
	description := "Bad Request: chat not found; token=secret"
	code, summary := classifyJobFailure(
		store.Job{Type: "deliver_lesson"},
		&telegram.APIError{Code: 400, Description: description},
		false,
	)
	if code != "telegram_chat_unavailable" {
		t.Fatalf("code = %q", code)
	}
	if strings.Contains(summary, description) || strings.Contains(summary, "secret") {
		t.Fatalf("Telegram description leaked into diagnostic: %q", summary)
	}
	if !strings.Contains(summary, "Reconnect the bot") {
		t.Fatalf("diagnostic is not actionable: %q", summary)
	}
}

func TestClassifyJobFailureUsesJobSpecificFallback(t *testing.T) {
	code, summary := classifyJobFailure(
		store.Job{Type: "ingest_source"},
		errors.New("private upstream response"),
		false,
	)
	if code != "source_ingestion_failed" ||
		summary != "The source feed could not be fetched or parsed." {
		t.Fatalf("unexpected fallback: %q %q", code, summary)
	}
}
