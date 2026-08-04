package jobs

import (
	"html"
	"reflect"
	"strings"
	"testing"

	"coreloop/backend/internal/radar"
	"coreloop/backend/internal/telegram"
)

func TestDeterministicRadarBriefingReachesMultipartTelegramContract(t *testing.T) {
	detail := strings.Repeat(
		"The release changes request routing, deployment compatibility, observability, and rollback behavior for production services. ",
		45,
	)
	briefing, err := radar.RenderBriefing(radar.BriefingInput{
		Category: radar.CategoryRelease,
		Title:    "Runtime 2.0 is generally available",
		Summary: "We are thrilled to announce a world-class release. " + detail +
			"Register now.",
		WhyItMatters: radarDeveloperContext(string(radar.CategoryRelease)),
		Source: radar.SourceReference{
			Name: "Runtime project", URL: "https://example.com/releases/2.0",
		},
		DiscoveredVia: []radar.SourceReference{{
			Name: "Hacker News discussion", URL: "https://news.ycombinator.com/item?id=42",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(briefing)
	for _, promotional := range []string{"thrilled", "world-class", "register now"} {
		if strings.Contains(lower, promotional) {
			t.Fatalf("briefing retained promotional phrase %q", promotional)
		}
	}
	if !strings.Contains(briefing, "https://example.com/releases/2.0") ||
		!strings.Contains(briefing, "https://news.ycombinator.com/item?id=42") {
		t.Fatalf("briefing is missing source provenance: %s", briefing)
	}
	parts := telegram.ChunkHTML([]string{html.EscapeString(briefing)}, "")
	if len(parts) < 2 {
		t.Fatalf("detailed Radar briefing was not split: %d part(s)", len(parts))
	}
	for index, part := range parts {
		if strings.TrimSpace(part) == "" {
			t.Fatalf("part %d is empty", index+1)
		}
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

func TestRadarEnrichmentCannotReplaceDetailWithAHeadline(t *testing.T) {
	source := strings.Repeat("Detailed source fact. ", 60)
	if radarEnrichmentDetailedEnough(source, "Short headline.") {
		t.Fatal("an underspecified enrichment replaced detailed source evidence")
	}
	if !radarEnrichmentDetailedEnough("Short source.", "A clear rewritten source explanation.") {
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
