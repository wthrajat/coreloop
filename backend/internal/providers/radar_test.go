package providers

import (
	"context"
	"errors"
	"testing"
)

func TestRadarEnrichmentFallsBackAcrossFreeProvidersOnly(t *testing.T) {
	groq := &fakeProvider{name: "groq", configured: true, responses: []any{
		&Error{Provider: "groq", Kind: FailureQuota, Message: "quota exhausted"},
	}}
	gemini := &fakeProvider{name: "gemini", configured: true, responses: []any{
		map[string]string{
			"simple_explanation": "The runtime now has a stable API and starts faster. This can make frequent deployments simpler.",
		},
	}}
	openAI := &fakeProvider{name: "openai", configured: true, responses: []any{
		errors.New("OpenAI must not run automatically"),
	}}
	enrichment, err := NewRouter(groq, gemini, openAI).EnrichRadar(
		context.Background(),
		RadarInput{Category: "release", Publisher: "Example", Title: "Runtime 2.0", Summary: "Adds a stable API."},
	)
	if err != nil {
		t.Fatal(err)
	}
	if enrichment.Provider != "gemini" || enrichment.Explanation == "" {
		t.Fatalf("enrichment = %#v", enrichment)
	}
	if openAI.calls != 0 {
		t.Fatal("OpenAI was called for automatic Radar enrichment")
	}
}

func TestRadarEnrichmentReportsFailureForDeterministicFallback(t *testing.T) {
	groq := &fakeProvider{name: "groq", configured: true, responses: []any{
		&Error{Provider: "groq", Kind: FailureQuota, Message: "quota exhausted"},
	}}
	_, err := NewRouter(groq, nil, nil).EnrichRadar(
		context.Background(), RadarInput{Title: "A release", Summary: "Details"},
	)
	if err == nil {
		t.Fatal("expected an enrichment miss")
	}
	if !errors.Is(err, ErrFreeQuotaExhausted) {
		t.Fatalf("error = %v, want free-quota classification", err)
	}
}

func TestRadarEnrichmentReportsMissingFreeProvider(t *testing.T) {
	_, err := NewRouter(nil, nil, &fakeProvider{
		name: "openai", configured: true,
	}).EnrichRadar(context.Background(), RadarInput{Title: "A release", Summary: "Details"})
	if !errors.Is(err, ErrNoFreeProviderConfigured) {
		t.Fatalf("error = %v, want missing free-provider classification", err)
	}
}
