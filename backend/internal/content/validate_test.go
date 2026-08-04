package content

import (
	"strings"
	"testing"
)

func TestValidateMarksUnverifiedClaims(t *testing.T) {
	draft := validDraft()
	draft.Claims = []Claim{{Text: "A changing fact", Status: "unverified"}}
	problems, state := Validate(draft, 15, nil)
	if len(problems) != 0 {
		t.Fatalf("unexpected problems: %v", problems)
	}
	if state != "unverified_warning" {
		t.Fatalf("state=%s", state)
	}
}

func TestValidateRequiresThirtyMinuteDepthAcrossSections(t *testing.T) {
	draft := validDraft()
	draft.EstimatedMinutes = 30
	draft.Definition = "A shallow definition."
	draft.Mechanics = []string{strings.Repeat("mechanical detail ", 180)}

	problems, _ := Validate(draft, 30, nil)
	for _, expected := range []string{
		"definition needs at least 180 words",
		"mechanics needs at least 4 developed points",
		"mechanics needs at least 600 words",
	} {
		if !containsValidationProblem(problems, expected) {
			t.Fatalf("validation problems are missing %q: %v", expected, problems)
		}
	}
}

func TestLessonWordMinimumScalesWithDuration(t *testing.T) {
	if got := minimumLessonWords(15); got != 1800 {
		t.Fatalf("15-minute minimum = %d, want 1800", got)
	}
	if got := minimumLessonWords(30); got != 3600 {
		t.Fatalf("30-minute minimum = %d, want 3600", got)
	}
}

func containsValidationProblem(problems []string, expected string) bool {
	for _, problem := range problems {
		if strings.Contains(problem, expected) {
			return true
		}
	}
	return false
}

func validDraft() LessonDraft {
	long := strings.Repeat("technical detail ", 180)
	developedList := []string{long, long, long, long}
	return LessonDraft{Title: "Title", EstimatedMinutes: 15, Motivation: long, PriorApproaches: developedList,
		Definition: long, Mechanics: developedList, ProductionExample: long, Tradeoffs: developedList,
		FailureModes: developedList, WhenNotToUse: developedList, Alternatives: developedList, Security: long,
		Reliability: long, Performance: long, Cost: long, PresentMaturity: long, FutureDirection: long,
		CareerRelevance: long, InterviewAnswer: long, RecallQuestion: "Question"}
}
