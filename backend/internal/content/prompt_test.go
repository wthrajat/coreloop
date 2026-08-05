package content

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCacheKeyIncludesThemeProgression(t *testing.T) {
	first := LessonContext{
		TopicID: "topic_databases", Position: 1,
		Objectives: []string{"Explain replication"},
	}
	fifth := first
	fifth.Position = 5
	fifth.CoveredObjectives = []string{"Explain replication", "Compare consistency models"}

	firstKey, err := CacheKey(first)
	if err != nil {
		t.Fatal(err)
	}
	fifthKey, err := CacheKey(fifth)
	if err != nil {
		t.Fatal(err)
	}
	if firstKey == fifthKey {
		t.Fatal("different theme positions must not reuse the same cached lesson")
	}
}

func TestCompileRequiresSimpleLanguageAndInlineDefinitions(t *testing.T) {
	system, input, err := Compile(LessonContext{
		Topic: "Database replication", Minutes: 30, Depth: "detailed",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, instruction := range []string{
		"short direct sentences",
		"define each unfamiliar technical term or acronym at first use in parentheses",
		"simple mental model",
	} {
		if !strings.Contains(system, instruction) {
			t.Fatalf("system instructions are missing %q", instruction)
		}
	}

	var dynamic struct {
		GenerationRequirements generationRequirements `json:"generation_requirements"`
	}
	if err := json.Unmarshal([]byte(input), &dynamic); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dynamic.GenerationRequirements.WordTarget, "3,800-4,500") {
		t.Fatalf("30-minute word target = %q", dynamic.GenerationRequirements.WordTarget)
	}
	if !strings.Contains(dynamic.GenerationRequirements.SectionDepth, "failure") {
		t.Fatalf("30-minute section guidance = %q", dynamic.GenerationRequirements.SectionDepth)
	}
	if !strings.Contains(dynamic.GenerationRequirements.ExplanationDepth, "internals") {
		t.Fatalf("detailed-mode guidance = %q", dynamic.GenerationRequirements.ExplanationDepth)
	}
}

func TestCompileKeepsFifteenMinuteGuidanceDetailedButSmaller(t *testing.T) {
	_, input, err := Compile(LessonContext{Minutes: 15, Depth: "foundation"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var dynamic struct {
		GenerationRequirements generationRequirements `json:"generation_requirements"`
	}
	if err := json.Unmarshal([]byte(input), &dynamic); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dynamic.GenerationRequirements.WordTarget, "1,900-2,400") {
		t.Fatalf("15-minute word target = %q", dynamic.GenerationRequirements.WordTarget)
	}
	if !strings.Contains(dynamic.GenerationRequirements.ExplanationDepth, "prerequisites") {
		t.Fatalf("foundation guidance = %q", dynamic.GenerationRequirements.ExplanationDepth)
	}
	if OutputBudget(30) <= OutputBudget(15) {
		t.Fatal("30-minute lessons need a larger output budget")
	}
}
