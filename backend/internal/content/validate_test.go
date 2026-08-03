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

func validDraft() LessonDraft {
	long := strings.Repeat("technical detail ", 180)
	return LessonDraft{Title: "Title", EstimatedMinutes: 15, Motivation: long, PriorApproaches: []string{long},
		Definition: long, Mechanics: []string{long}, ProductionExample: long, Tradeoffs: []string{long},
		FailureModes: []string{long}, WhenNotToUse: []string{long}, Alternatives: []string{long}, Security: long,
		Reliability: long, Performance: long, Cost: long, PresentMaturity: long, FutureDirection: long,
		CareerRelevance: long, InterviewAnswer: long, RecallQuestion: "Question"}
}
