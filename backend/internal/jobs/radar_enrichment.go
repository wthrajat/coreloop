package jobs

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"log/slog"

	"coreloop/backend/internal/providers"
	"coreloop/backend/internal/radar"
	"coreloop/backend/internal/store"
)

const radarEnrichmentVersion = "simple-explanation-v3"
const maximumRadarAIInputRunes = 6_000

const (
	radarSummaryNoProvider = "AI summary unavailable: no free AI provider is configured."
	radarSummaryQuota      = "AI summary unavailable: free AI quota is exhausted."
	radarSummaryTemporary  = "AI summary unavailable right now."
	radarSummaryNoDetail   = "AI summary unavailable: the source did not provide enough detail to simplify safely."
)

type radarBriefingSections struct {
	SourceSummary     string
	DeveloperContext  string
	SimpleExplanation string
}

// radarBriefingContent is deliberately fail-open. Every cache, provider,
// validation, and persistence failure returns deterministic content rather than
// an error to the durable job worker.
func (service *Service) radarBriefingContent(
	ctx context.Context,
	candidate store.RadarCandidate,
) radarBriefingSections {
	deterministicSummary := candidate.Summary
	deterministicContext := radarDeveloperContext(candidate.Category)
	sections := radarBriefingSections{
		SourceSummary:    deterministicSummary,
		DeveloperContext: deterministicContext,
	}
	if deterministicSummary == "" {
		sections.SimpleExplanation = radarSummaryNoDetail
		return sections
	}
	if service.providers == nil {
		sections.SimpleExplanation = radarSummaryNoProvider
		return sections
	}
	inputHash := radarEnrichmentHash(candidate)
	cached, err := service.store.RadarEnrichment(
		ctx, candidate.SourceItemID, inputHash, radarEnrichmentVersion,
	)
	if err == nil {
		slog.InfoContext(ctx, "Radar enrichment cache hit", "source_item_id", candidate.SourceItemID)
		sections.SimpleExplanation = cached.Summary
		return sections
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.WarnContext(ctx, "Radar enrichment cache unavailable", "error", err)
	}
	enrichment, err := service.providers.EnrichRadar(ctx, providers.RadarInput{
		Category: candidate.Category, Publisher: candidate.Publisher,
		Title: candidate.Title, Summary: compactRadarAIInput(deterministicSummary),
	})
	if err != nil {
		reason := "provider_failure"
		sections.SimpleExplanation = radarSummaryTemporary
		switch {
		case errors.Is(err, providers.ErrFreeQuotaExhausted):
			reason = "free_quota_exhausted"
			sections.SimpleExplanation = radarSummaryQuota
		case errors.Is(err, providers.ErrNoFreeProviderConfigured):
			reason = "no_free_provider"
			sections.SimpleExplanation = radarSummaryNoProvider
		}
		slog.InfoContext(ctx, "Radar AI summary unavailable; source-backed delivery continues",
			"reason", reason, "source_item_id", candidate.SourceItemID)
		return sections
	}
	enrichment.Explanation = radar.NeutralText(enrichment.Explanation)
	if !radarExplanationDetailedEnough(deterministicSummary, enrichment.Explanation) {
		sections.SimpleExplanation = radarSummaryTemporary
		slog.InfoContext(ctx, "Radar AI summary rejected; source-backed delivery continues",
			"reason", "explanation_too_shallow", "source_item_id", candidate.SourceItemID)
		return sections
	}
	slog.InfoContext(ctx, "Radar AI enrichment completed", "provider", enrichment.Provider,
		"model", enrichment.Model, "source_item_id", candidate.SourceItemID)
	cacheValue := store.RadarEnrichment{
		Summary: enrichment.Explanation, WhyItMatters: "",
		Provider: enrichment.Provider, Model: enrichment.Model,
	}
	if err := service.store.SaveRadarEnrichment(
		ctx, candidate.SourceItemID, inputHash, radarEnrichmentVersion,
		cacheValue, service.now(),
	); err != nil {
		slog.WarnContext(ctx, "Radar enrichment could not be cached", "error", err)
	}
	sections.SimpleExplanation = cacheValue.Summary
	return sections
}

func compactRadarAIInput(value string) string {
	runes := []rune(value)
	if len(runes) <= maximumRadarAIInputRunes {
		return value
	}
	return string(runes[:maximumRadarAIInputRunes])
}

func radarExplanationDetailedEnough(sourceSummary, explanation string) bool {
	explanationLength := len([]rune(explanation))
	if explanationLength == 0 {
		return false
	}
	minimum := min(120, max(40, len([]rune(sourceSummary))/5))
	return explanationLength >= minimum
}

func radarEnrichmentHash(candidate store.RadarCandidate) string {
	digest := sha256.Sum256([]byte(
		candidate.Category + "\n" + candidate.Publisher + "\n" +
			candidate.Title + "\n" + candidate.Summary,
	))
	return hex.EncodeToString(digest[:])
}
