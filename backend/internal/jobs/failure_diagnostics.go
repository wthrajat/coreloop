package jobs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"strings"

	"coreloop/backend/internal/providers"
	"coreloop/backend/internal/store"
	"coreloop/backend/internal/telegram"
)

func classifyJobFailure(job store.Job, failure error, quota bool) (string, string) {
	if quota || errors.Is(failure, providers.ErrFreeQuotaExhausted) {
		return "ai_quota_exhausted",
			"Every configured free AI provider is unavailable or out of quota. The job will remain blocked until free capacity returns."
	}
	if errors.Is(failure, store.ErrIncompleteLesson) {
		return "incomplete_lesson",
			"The generated lesson failed the required completeness and depth checks."
	}
	if telegram.IsChatUnavailable(failure) {
		return "telegram_chat_unavailable",
			"Telegram cannot deliver to the configured chat. Reconnect the bot and confirm the conversation is still available."
	}
	var telegramFailure *telegram.APIError
	if errors.As(failure, &telegramFailure) {
		return "telegram_api_error", fmt.Sprintf(
			"Telegram rejected the delivery request (API code %d).",
			telegramFailure.Code,
		)
	}
	if providerFailures := collectProviderFailures(failure); len(providerFailures) > 0 {
		return summarizeProviderFailures(providerFailures)
	}
	if errors.Is(failure, context.DeadlineExceeded) {
		return "execution_timeout",
			"The job exceeded its execution deadline before it could finish."
	}
	if errors.Is(failure, context.Canceled) {
		return "execution_cancelled",
			"The job execution was cancelled before it could finish."
	}
	var networkFailure net.Error
	if errors.As(failure, &networkFailure) && networkFailure.Timeout() {
		return "upstream_timeout",
			"An upstream service did not respond before the network timeout."
	}
	if errors.Is(failure, sql.ErrNoRows) {
		return "required_record_missing",
			"A database record required by this job could not be found."
	}
	return fallbackJobFailure(job.Type)
}

func collectProviderFailures(failure error) []*providers.Error {
	var collected []*providers.Error
	seen := make(map[string]bool)
	var visit func(error)
	visit = func(current error) {
		if current == nil {
			return
		}
		if providerFailure, ok := current.(*providers.Error); ok {
			key := fmt.Sprintf(
				"%s:%s:%d",
				providerFailure.Provider,
				providerFailure.Kind,
				providerFailure.Status,
			)
			if !seen[key] {
				seen[key] = true
				collected = append(collected, providerFailure)
			}
		}
		switch wrapped := current.(type) {
		case interface{ Unwrap() []error }:
			for _, child := range wrapped.Unwrap() {
				visit(child)
			}
		case interface{ Unwrap() error }:
			visit(wrapped.Unwrap())
		}
	}
	visit(failure)
	return collected
}

func summarizeProviderFailures(failures []*providers.Error) (string, string) {
	details := make([]string, 0, len(failures))
	code := "ai_provider_failed"
	for _, failure := range failures {
		providerName := providerDisplayName(failure.Provider)
		status := ""
		if failure.Status > 0 {
			status = fmt.Sprintf(" (HTTP %d)", failure.Status)
		}
		switch failure.Kind {
		case providers.FailureInvalid:
			code = "ai_invalid_output"
			details = append(details, providerName+" returned invalid lesson output"+status)
		case providers.FailureQuota:
			details = append(details, providerName+" reported a quota limit"+status)
		case providers.FailureTransient:
			details = append(details, providerName+" had a temporary provider failure"+status)
		case providers.FailurePermanent:
			details = append(details, providerName+" rejected the request"+status)
		default:
			details = append(details, providerName+" failed"+status)
		}
	}
	return code, strings.Join(details, "; ") + "."
}

func providerDisplayName(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "groq":
		return "Groq"
	case "gemini":
		return "Gemini"
	case "openai":
		return "OpenAI"
	default:
		return "An AI provider"
	}
}

func fallbackJobFailure(jobType string) (string, string) {
	switch jobType {
	case "generate_lesson":
		return "lesson_generation_failed", "Lesson generation failed before valid content could be saved."
	case "deliver_lesson":
		return "lesson_delivery_failed", "The saved lesson could not be delivered through Telegram."
	case "ingest_source":
		return "source_ingestion_failed", "The source feed could not be fetched or parsed."
	case "rank_radar":
		return "radar_ranking_failed", "The latest Radar candidates could not be ranked."
	case "deliver_radar":
		return "radar_delivery_failed", "The selected Radar update could not be delivered through Telegram."
	case "recover":
		return "queue_recovery_failed", "Durable queue recovery did not complete."
	default:
		return "job_failed", "The job failed before it could complete."
	}
}
