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

func TestRadarReleaseIntervalSpreadsFiniteTargetAcrossTheDay(t *testing.T) {
	if got := radarReleaseInterval(8); got != 3*time.Hour {
		t.Fatalf("8-item interval = %s", got)
	}
	if got := radarReleaseInterval(0); got != 0 {
		t.Fatalf("unlimited interval = %s", got)
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
