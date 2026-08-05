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

func TestDeliveryReadyRejectsHeadingOnlyTailSections(t *testing.T) {
	draft := validDraft()
	draft.ProductionExample = ""
	draft.Tradeoffs = []string{"", "", ""}
	draft.FailureModes = nil
	draft.Security = ""

	if DeliveryReady(draft, 15) {
		t.Fatal("heading-only lesson was considered ready for delivery")
	}
	problems := ContentProblems(draft, 15)
	for _, expected := range []string{
		"production_example is empty",
		"tradeoffs is empty",
		"tradeoffs contains empty points",
		"failure_modes is empty",
		"security is empty",
	} {
		if !containsValidationProblem(problems, expected) {
			t.Fatalf("validation problems are missing %q: %v", expected, problems)
		}
	}
}

func TestDeliveryReadyEnforcesThirtyMinuteReadingDepth(t *testing.T) {
	short := strings.Repeat("detail ", 100)
	developedList := []string{short, short, short, short}
	draft := LessonDraft{
		Title: "Title", EstimatedMinutes: 30, Motivation: short,
		PriorApproaches: developedList, Definition: short, Mechanics: developedList,
		ProductionExample: short, Tradeoffs: developedList, FailureModes: developedList,
		WhenNotToUse: developedList, Alternatives: developedList, Security: short,
		Reliability: short, Performance: short, Cost: short, PresentMaturity: short,
		FutureDirection: short, CareerRelevance: short, InterviewAnswer: short,
		RecallQuestion: "Question",
	}
	if DeliveryReady(draft, 30) {
		t.Fatal("a short draft was considered a complete 30-minute lesson")
	}
	if !containsValidationProblem(ContentProblems(draft, 30), "lesson is too short") {
		t.Fatal("30-minute lesson did not report its total word-count failure")
	}
}

func TestRenderSectionsNeverCreatesHeadingOnlyMessages(t *testing.T) {
	draft := LessonDraft{
		Title: "Partial lesson", EstimatedMinutes: 15,
		Motivation: "A developed opening.",
	}
	sections := RenderSections(draft)
	for _, forbidden := range []string{"Production scenario", "Trade-offs", "Failure modes"} {
		if containsString(sections, forbidden) {
			t.Fatalf("empty %q heading reached rendering: %v", forbidden, sections)
		}
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
