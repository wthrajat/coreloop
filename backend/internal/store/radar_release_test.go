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
	want := []string{"aws-1", "aws-2", "hn-1", "openai-1"}
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

func TestDiverseRadarCandidatesAppliesASoftDailySourcePenalty(t *testing.T) {
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
