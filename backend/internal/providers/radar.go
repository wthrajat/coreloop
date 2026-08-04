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
	radarOutputBudget    = 600
)

type RadarInput struct {
	Category  string `json:"category"`
	Publisher string `json:"publisher"`
	Title     string `json:"title"`
	Summary   string `json:"summary"`
}

type RadarEnrichment struct {
	Summary      string `json:"what_changed"`
	WhyItMatters string `json:"why_it_matters"`
	Provider     string `json:"-"`
	Model        string `json:"-"`
}

// EnrichRadar tries configured free providers with a deliberately small input
// and output. Callers must treat every error as a cache miss and continue with
// deterministic rendering.
func (router *Router) EnrichRadar(ctx context.Context, input RadarInput) (RadarEnrichment, error) {
	if router == nil {
		return RadarEnrichment{}, errors.New("AI provider router is unavailable")
	}
	encodedInput, err := json.Marshal(input)
	if err != nil {
		return RadarEnrichment{}, err
	}
	system := "Rewrite one important technology-news item in simple, neutral English. " +
		"Use only facts present in the supplied title and summary. Do not add claims, " +
		"predictions, praise, calls to action, or links. Be concise: for an application " +
		"or library version release, use at most two short what_changed sentences and one " +
		"short why_it_matters sentence. Use extra detail only for a consequential incident, " +
		"genuinely new capability, substantial engineering analysis, or important research. " +
		"why_it_matters should explain what a developer can learn or may need to change " +
		"without claiming facts absent from the source."
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"what_changed":   map[string]any{"type": "string"},
			"why_it_matters": map[string]any{"type": "string"},
		},
		"required":             []string{"what_changed", "why_it_matters"},
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
		enrichment.Summary = strings.TrimSpace(enrichment.Summary)
		enrichment.WhyItMatters = strings.TrimSpace(enrichment.WhyItMatters)
		if enrichment.Summary == "" || len([]rune(enrichment.Summary)) > 6_000 || len([]rune(enrichment.WhyItMatters)) > 2_500 {
			failures = append(failures, fmt.Errorf("%s Radar output is outside accepted bounds", provider.Name()))
			continue
		}
		enrichment.Provider, enrichment.Model = provider.Name(), provider.Model()
		return enrichment, nil
	}
	if len(failures) == 0 {
		return RadarEnrichment{}, errors.New("no free AI provider is configured")
	}
	return RadarEnrichment{}, errors.Join(failures...)
}
