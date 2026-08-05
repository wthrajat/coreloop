package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	radarProviderTimeout = 5 * time.Second
	radarOutputBudget    = 400
)

type RadarInput struct {
	Category  string `json:"category"`
	Publisher string `json:"publisher"`
	Title     string `json:"title"`
	Summary   string `json:"summary"`
}

type RadarEnrichment struct {
	Explanation string `json:"simple_explanation"`
	Provider    string `json:"-"`
	Model       string `json:"-"`
}

// EnrichRadar tries configured free providers with a deliberately small input
// and output. Callers must treat every error as a cache miss and continue with
// deterministic rendering.
func (router *Router) EnrichRadar(ctx context.Context, input RadarInput) (RadarEnrichment, error) {
	if router == nil {
		return RadarEnrichment{}, ErrNoFreeProviderConfigured
	}
	encodedInput, err := json.Marshal(input)
	if err != nil {
		return RadarEnrichment{}, err
	}
	system := "Explain one technology-news item in very simple, neutral English. " +
		"Use only facts present in the supplied title and summary. Write two to four short " +
		"sentences explaining what happened and why a developer may care. Briefly define an " +
		"unfamiliar technical term in parentheses when useful. Do not add claims, predictions, " +
		"praise, calls to action, or links. Keep routine version releases especially short."
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"simple_explanation": map[string]any{"type": "string"},
		},
		"required":             []string{"simple_explanation"},
		"additionalProperties": false,
	}
	var failures []error
	for _, provider := range router.free {
		if provider == nil || !provider.Configured() {
			continue
		}
		attemptContext, cancel := context.WithTimeout(ctx, radarProviderTimeout)
		response, generateErr := provider.Generate(
			attemptContext, system, string(encodedInput), schema, radarOutputBudget,
		)
		cancel()
		if generateErr != nil {
			failures = append(failures, generateErr)
			continue
		}
		var enrichment RadarEnrichment
		if err := json.Unmarshal(response.Body, &enrichment); err != nil {
			failures = append(failures, fmt.Errorf("%s Radar output is invalid JSON: %w", provider.Name(), err))
			continue
		}
		enrichment.Explanation = strings.TrimSpace(enrichment.Explanation)
		if enrichment.Explanation == "" || len([]rune(enrichment.Explanation)) > 2_500 {
			failures = append(failures, fmt.Errorf("%s Radar output is outside accepted bounds", provider.Name()))
			continue
		}
		enrichment.Provider, enrichment.Model = provider.Name(), provider.Model()
		return enrichment, nil
	}
	if len(failures) == 0 {
		return RadarEnrichment{}, ErrNoFreeProviderConfigured
	}
	joined := errors.Join(failures...)
	if everyFailureIsQuotaExhaustion(failures) {
		return RadarEnrichment{}, fmt.Errorf("%w: %v", ErrFreeQuotaExhausted, joined)
	}
	return RadarEnrichment{}, joined
}
