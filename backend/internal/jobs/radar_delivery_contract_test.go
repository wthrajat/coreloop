package jobs

import (
	"html"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"coreloop/backend/internal/radar"
	"coreloop/backend/internal/store"
	"coreloop/backend/internal/telegram"
)

func TestDeterministicRadarBriefingAlwaysFitsOneTelegramMessage(t *testing.T) {
	detail := strings.Repeat(
		"The release changes request routing, deployment compatibility, observability, and rollback behavior for production services. ",
		45,
	)
	briefing, err := radar.RenderCompactBriefing(radar.BriefingInput{
		Category: radar.CategoryRelease,
		Title:    "Runtime 2.0 is generally available",
		Summary: "We are thrilled to announce a world-class release. " + detail +
			"Register now.",
		WhyItMatters:      radarDeveloperContext(string(radar.CategoryRelease)),
		SimpleExplanation: "This runtime changed how requests are routed. Developers should review compatibility before upgrading.",
		Source: radar.SourceReference{
			Name: "Runtime project", URL: "https://example.co/releases/2.0",
		},
		DiscoveredVia: []radar.SourceReference{{
			Name: "Hacker News discussion", URL: "https://news.ycombinator.com/item?id=42",
		}},
	}, telegram.SafeChunkCharacters)
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(briefing)
	for _, promotional := range []string{"thrilled", "world-class", "register now"} {
		if strings.Contains(lower, promotional) {
			t.Fatalf("briefing retained promotional phrase %q", promotional)
		}
	}
	if !strings.Contains(briefing, "https://example.co/releases/2.0") ||
		!strings.Contains(briefing, "https://news.ycombinator.com/item?id=42") {
		t.Fatalf("briefing is missing source provenance: %s", briefing)
	}
	if !strings.Contains(briefing, "Simple explanation") {
		t.Fatalf("briefing lost the optional AI explanation: %s", briefing)
	}
	message := html.EscapeString(briefing)
	if strings.TrimSpace(message) == "" {
		t.Fatal("Radar message is empty")
	}
	if got := utf8.RuneCountInString(message); got > telegram.SafeChunkCharacters {
		t.Fatalf("Radar message has %d escaped runes, limit %d", got, telegram.SafeChunkCharacters)
	}
}

func TestRadarRankWorkIsSplitIntoServerlessSizedBatches(t *testing.T) {
	itemIDs := []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11"}
	got := splitRadarItemIDs(itemIDs, 5)
	want := [][]string{{"1", "2", "3", "4", "5"}, {"6", "7", "8", "9", "10"}, {"11"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("batches = %#v, want %#v", got, want)
	}
}

func TestRadarSimpleExplanationMustContainUsefulDetail(t *testing.T) {
	source := strings.Repeat("Detailed source fact. ", 60)
	if radarExplanationDetailedEnough(source, "Short headline.") {
		t.Fatal("an underspecified explanation was accepted")
	}
	if !radarExplanationDetailedEnough(
		"Short source.",
		"This is a clear, simple explanation of the source and why a developer may care.",
	) {
		t.Fatal("a sufficiently detailed enrichment was rejected")
	}
}

func TestRadarAIInputIsBounded(t *testing.T) {
	source := strings.Repeat("Detailed source fact. ", 1_000)
	compact := compactRadarAIInput(source)
	if len([]rune(compact)) != maximumRadarAIInputRunes {
		t.Fatalf("compact input has %d runes", len([]rune(compact)))
	}
	if len([]rune(source)) <= len([]rune(compact)) {
		t.Fatal("test source was not longer than the AI input budget")
	}
}

func TestRadarDeliveryRechecksRankingPolicyAndFreshness(t *testing.T) {
	now := time.Date(2026, time.August, 20, 12, 0, 0, 0, time.UTC)
	candidate := store.RadarCandidate{
		RankerVersion: radar.RankerVersion,
		Score:         radar.MinimumDeliveryScore,
		PublishedAt:   now.Add(-radar.MaximumItemAge),
		URL:           "https://example.co/current-update",
	}
	if reason := radarDeliveryRejectionReason(candidate, now); reason != "" {
		t.Fatalf("boundary candidate rejected: %s", reason)
	}

	candidate.PublishedAt = candidate.PublishedAt.Add(-time.Second)
	if reason := radarDeliveryRejectionReason(candidate, now); reason != "outside_freshness_window" {
		t.Fatalf("stale candidate reason = %q", reason)
	}

	candidate.PublishedAt = now
	candidate.RankerVersion = "deterministic-editorial-v2"
	if reason := radarDeliveryRejectionReason(candidate, now); reason != "superseded_ranking_policy" {
		t.Fatalf("old-ranker candidate reason = %q", reason)
	}

	candidate.RankerVersion = radar.RankerVersion
	candidate.Score = radar.MinimumDeliveryScore - 0.001
	if reason := radarDeliveryRejectionReason(candidate, now); reason != "below_editorial_threshold" {
		t.Fatalf("low-score candidate reason = %q", reason)
	}

	candidate.Score = radar.MinimumDeliveryScore
	candidate.URL = "https://example.com/placeholder"
	if reason := radarDeliveryRejectionReason(candidate, now); reason != "invalid_source_url" {
		t.Fatalf("placeholder-source candidate reason = %q", reason)
	}
}
