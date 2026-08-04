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

const radarEnrichmentVersion = "simple-neutral-v2"
const maximumRadarAIInputRunes = 6_000

// radarBriefingContent is deliberately fail-open. Every cache, provider,
// validation, and persistence failure returns deterministic content rather than
// an error to the durable job worker.
func (service *Service) radarBriefingContent(
	ctx context.Context,
	candidate store.RadarCandidate,
) (string, string) {
	deterministicSummary := candidate.Summary
	deterministicContext := radarDeveloperContext(candidate.Category)
	if service.providers == nil || deterministicSummary == "" {
		return deterministicSummary, deterministicContext
	}
	inputHash := radarEnrichmentHash(candidate)
	cached, err := service.store.RadarEnrichment(
		ctx, candidate.SourceItemID, inputHash, radarEnrichmentVersion,
	)
	if err == nil {
		slog.InfoContext(ctx, "Radar enrichment cache hit", "source_item_id", candidate.SourceItemID)
		return cached.Summary, cached.WhyItMatters
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.WarnContext(ctx, "Radar enrichment cache unavailable", "error", err)
	}
	enrichment, err := service.providers.EnrichRadar(ctx, providers.RadarInput{
		Category: candidate.Category, Publisher: candidate.Publisher,
		Title: candidate.Title, Summary: compactRadarAIInput(deterministicSummary),
	})
	if err != nil {
		slog.InfoContext(ctx, "Radar AI enrichment unavailable; deterministic fallback selected", "error", err)
		return deterministicSummary, deterministicContext
	}
	enrichment.Summary = radar.NeutralText(enrichment.Summary)
	enrichment.WhyItMatters = radar.NeutralText(enrichment.WhyItMatters)
	if !radarEnrichmentDetailedEnough(deterministicSummary, enrichment.Summary) {
		return deterministicSummary, deterministicContext
	}
	if enrichment.WhyItMatters == "" {
		enrichment.WhyItMatters = deterministicContext
	}
	slog.InfoContext(ctx, "Radar AI enrichment completed", "provider", enrichment.Provider,
		"model", enrichment.Model, "source_item_id", candidate.SourceItemID)
	cacheValue := store.RadarEnrichment{
		Summary: enrichment.Summary, WhyItMatters: enrichment.WhyItMatters,
		Provider: enrichment.Provider, Model: enrichment.Model,
	}
	if err := service.store.SaveRadarEnrichment(
		ctx, candidate.SourceItemID, inputHash, radarEnrichmentVersion,
		cacheValue, service.now(),
	); err != nil {
		slog.WarnContext(ctx, "Radar enrichment could not be cached", "error", err)
	}
	return cacheValue.Summary, cacheValue.WhyItMatters
}

func compactRadarAIInput(value string) string {
	runes := []rune(value)
	if len(runes) <= maximumRadarAIInputRunes {
		return value
	}
	return string(runes[:maximumRadarAIInputRunes])
}

func radarEnrichmentDetailedEnough(sourceSummary, enrichedSummary string) bool {
	enrichedLength := len([]rune(enrichedSummary))
	if enrichedLength == 0 {
		return false
	}
	minimum := len([]rune(sourceSummary)) / 3
	return enrichedLength >= minimum
}

func radarEnrichmentHash(candidate store.RadarCandidate) string {
	digest := sha256.Sum256([]byte(
		candidate.Category + "\n" + candidate.Publisher + "\n" +
			candidate.Title + "\n" + candidate.Summary,
	))
	return hex.EncodeToString(digest[:])
}
