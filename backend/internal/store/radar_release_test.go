package store

import (
	"reflect"
	"testing"
	"time"
)

func TestDiverseRadarCandidatesPrefersSourceBreadth(t *testing.T) {
	pool := []radarReleaseCandidate{
		{ID: "aws-1", SourceID: "aws", Score: 0.9},
		{ID: "aws-2", SourceID: "aws", Score: 0.88},
		{ID: "aws-3", SourceID: "aws", Score: 0.86},
		{ID: "hn-1", SourceID: "hn", Score: 0.8},
		{ID: "openai-1", SourceID: "openai", Score: 0.78},
	}
	got := diverseRadarCandidates(pool, nil, 4)
	want := []string{"aws-1", "hn-1", "openai-1", "aws-2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("selected = %#v, want %#v", got, want)
	}
}

func TestRadarReleaseSlotsStayAnchoredWhenCronRunsLate(t *testing.T) {
	location := time.FixedZone("Asia/Kolkata", 5*60*60+30*60)
	lastReleased := time.Date(2026, time.August, 5, 10, 9, 0, 0, location)
	// Twenty daily items create 72-minute slots. The current slot began at
	// 10:48, so an 11:10 cron tick is due even though the previous release was
	// only 61 minutes ago. A last-release-plus-interval calculation would drift.
	localNow := time.Date(2026, time.August, 5, 11, 10, 0, 0, location)
	if !radarReleaseSlotDue(lastReleased, localNow, 20) {
		t.Fatal("late cron tick did not release in the current deterministic slot")
	}
}

func TestRadarReleaseSlotsAllowOnlyOneItemPerSlot(t *testing.T) {
	location := time.FixedZone("Asia/Kolkata", 5*60*60+30*60)
	localNow := time.Date(2026, time.August, 5, 11, 10, 0, 0, location)
	lastReleased := time.Date(2026, time.August, 5, 10, 55, 0, 0, location)
	if radarReleaseSlotDue(lastReleased, localNow, 20) {
		t.Fatal("a second item was allowed in the same Radar slot")
	}
	if !radarReleaseSlotDue(lastReleased, localNow, 0) {
		t.Fatal("unlimited Radar should not apply finite slot spacing")
	}
}

func TestRadarReleaseSlotsResetAtTheLocalDayBoundary(t *testing.T) {
	location := time.FixedZone("Asia/Kolkata", 5*60*60+30*60)
	lastReleased := time.Date(2026, time.August, 4, 23, 58, 0, 0, location)
	localNow := time.Date(2026, time.August, 5, 0, 4, 0, 0, location)
	if !radarReleaseSlotDue(lastReleased, localNow, 20) {
		t.Fatal("the first Radar slot of a new local day was not released")
	}
}

func TestRadarCandidatesExpireAfterTenDays(t *testing.T) {
	if radarCandidateMaxAge != 10*24*time.Hour {
		t.Fatalf("Radar candidate max age = %s, want 10 days", radarCandidateMaxAge)
	}
}

func TestDiverseRadarCandidatesStillFillsFromOneSource(t *testing.T) {
	pool := []radarReleaseCandidate{
		{ID: "source-1", SourceID: "source", Score: 0.9},
		{ID: "source-2", SourceID: "source", Score: 0.8},
		{ID: "source-3", SourceID: "source", Score: 0.7},
	}
	got := diverseRadarCandidates(pool, nil, 3)
	want := []string{"source-1", "source-2", "source-3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("selected = %#v, want %#v", got, want)
	}
}

func TestDiverseRadarCandidatesPrefersTheLessUsedDailyFamily(t *testing.T) {
	pool := []radarReleaseCandidate{
		{ID: "aws", SourceID: "aws", Score: 0.9},
		{ID: "openai", SourceID: "openai", Score: 0.8},
	}
	got := diverseRadarCandidates(pool, map[string]int{"aws": 2}, 1)
	want := []string{"openai"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("selected = %#v, want %#v", got, want)
	}
}

func TestDiverseRadarCandidatesBalancesAFullBatchAcrossSourceFamilies(t *testing.T) {
	pool := []radarReleaseCandidate{
		{ID: "arxiv-1", SourceID: "source_arxiv_ai", Score: 0.99},
		{ID: "arxiv-2", SourceID: "source_arxiv_ml", Score: 0.98},
		{ID: "arxiv-3", SourceID: "source_arxiv_ai", Score: 0.97},
		{ID: "arxiv-4", SourceID: "source_arxiv_ml", Score: 0.96},
		{ID: "hn-1", SourceID: "source_hacker_news", Score: 0.89},
		{ID: "hn-2", SourceID: "source_hacker_news", Score: 0.88},
		{ID: "hn-3", SourceID: "source_hacker_news", Score: 0.87},
		{ID: "hn-4", SourceID: "source_hacker_news", Score: 0.86},
		{ID: "stacker-1", SourceID: "source_stacker_tech", Score: 0.85},
		{ID: "stacker-2", SourceID: "source_stacker_bitcoin", Score: 0.84},
		{ID: "stacker-3", SourceID: "source_stacker_lightning", Score: 0.83},
		{ID: "stacker-4", SourceID: "source_stacker_news", Score: 0.82},
		{ID: "openai-1", SourceID: "source_openai_news", Score: 0.81},
		{ID: "openai-2", SourceID: "source_openai_news", Score: 0.80},
		{ID: "openai-3", SourceID: "source_openai_news", Score: 0.79},
		{ID: "openai-4", SourceID: "source_openai_news", Score: 0.78},
		{ID: "cloudflare-1", SourceID: "source_cloudflare_blog", Score: 0.77},
		{ID: "cloudflare-2", SourceID: "source_cloudflare_blog", Score: 0.76},
		{ID: "cloudflare-3", SourceID: "source_cloudflare_blog", Score: 0.75},
		{ID: "cloudflare-4", SourceID: "source_cloudflare_blog", Score: 0.74},
	}

	got := diverseRadarCandidates(pool, nil, 20)
	if len(got) != 20 {
		t.Fatalf("selected = %d, want 20", len(got))
	}
	firstRound := got[:5]
	wantFirstRound := []string{
		"arxiv-1", "hn-1", "stacker-1", "openai-1", "cloudflare-1",
	}
	if !reflect.DeepEqual(firstRound, wantFirstRound) {
		t.Fatalf("first family round = %#v, want %#v", firstRound, wantFirstRound)
	}
	counts := map[string]int{}
	byID := map[string]radarReleaseCandidate{}
	for _, candidate := range pool {
		byID[candidate.ID] = candidate
	}
	for _, id := range got {
		counts[radarSourceFamily(byID[id].SourceID)]++
	}
	for _, family := range []string{
		"arxiv", "hacker_news", "stacker_news",
		"source_openai_news", "source_cloudflare_blog",
	} {
		if counts[family] != 4 {
			t.Fatalf("family %q count = %d, want 4: %#v", family, counts[family], got)
		}
	}
}

func TestDiverseRadarCandidatesPrioritizesUnusedFamiliesAcrossTheDay(t *testing.T) {
	pool := []radarReleaseCandidate{
		{ID: "arxiv", SourceID: "source_arxiv_ai", Score: 0.99},
		{ID: "hn", SourceID: "source_hacker_news", Score: 0.76},
		{ID: "stacker", SourceID: "source_stacker_tech", Score: 0.74},
	}
	historicalUsage := map[string]int{"arxiv": 1, "hacker_news": 1}
	got := diverseRadarCandidates(pool, historicalUsage, 1)
	if !reflect.DeepEqual(got, []string{"stacker"}) {
		t.Fatalf("selected = %#v, want the unused Stacker News family", got)
	}
}

func TestRadarSourceFamilyCombinesHighVolumeSiblings(t *testing.T) {
	cases := map[string]string{
		"source_arxiv_ai":          "arxiv",
		"source_arxiv_ml":          "arxiv",
		"source_stacker_tech":      "stacker_news",
		"source_stacker_lightning": "stacker_news",
		"source_hacker_news":       "hacker_news",
		"source_openai_news":       "source_openai_news",
	}
	for sourceID, want := range cases {
		if got := radarSourceFamily(sourceID); got != want {
			t.Fatalf("source family for %q = %q, want %q", sourceID, got, want)
		}
	}
}
