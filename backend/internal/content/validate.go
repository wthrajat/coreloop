package content

import (
	"fmt"
	"net/url"
	"strings"
)

func Validate(draft LessonDraft, requestedMinutes int, allowedEvidence []Evidence) ([]string, string) {
	var problems []string
	requiredStrings := map[string]string{
		"title": draft.Title, "motivation": draft.Motivation, "definition": draft.Definition,
		"production_example": draft.ProductionExample, "security": draft.Security, "reliability": draft.Reliability,
		"performance": draft.Performance, "cost": draft.Cost, "present_maturity": draft.PresentMaturity,
		"future_direction": draft.FutureDirection, "career_relevance": draft.CareerRelevance,
		"interview_answer": draft.InterviewAnswer, "recall_question": draft.RecallQuestion,
	}
	for name, value := range requiredStrings {
		if strings.TrimSpace(value) == "" {
			problems = append(problems, name+" is empty")
		}
	}
	requiredLists := map[string][]string{"prior_approaches": draft.PriorApproaches, "mechanics": draft.Mechanics,
		"tradeoffs": draft.Tradeoffs, "failure_modes": draft.FailureModes, "when_not_to_use": draft.WhenNotToUse,
		"alternatives": draft.Alternatives}
	for name, values := range requiredLists {
		if len(values) == 0 {
			problems = append(problems, name+" is empty")
		}
	}
	if requestedMinutes != 15 && requestedMinutes != 30 {
		requestedMinutes = 15
	}
	if draft.EstimatedMinutes < requestedMinutes-5 || draft.EstimatedMinutes > requestedMinutes+10 {
		problems = append(problems, fmt.Sprintf("estimated_minutes %d does not match the %d-minute preset", draft.EstimatedMinutes, requestedMinutes))
	}
	wordCount := lessonWordCount(draft)
	minimumWords := minimumLessonWords(requestedMinutes)
	if wordCount < minimumWords {
		problems = append(problems, fmt.Sprintf(
			"lesson is too short: %d words; the %d-minute preset needs at least %d",
			wordCount, requestedMinutes, minimumWords,
		))
	}
	problems = append(problems, validateSectionDepth(draft, requestedMinutes)...)
	allowed := make(map[string]Evidence)
	for _, evidence := range allowedEvidence {
		allowed[evidence.ID] = evidence
	}
	sourceIDs := make(map[string]bool)
	for _, source := range draft.Sources {
		if _, exists := allowed[source.ID]; !exists {
			problems = append(problems, "source "+source.ID+" was not supplied")
		}
		parsed, err := url.Parse(source.CanonicalURL)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			problems = append(problems, "source "+source.ID+" has an invalid URL")
		}
		sourceIDs[source.ID] = true
	}
	verification := "verified"
	for index, claim := range draft.Claims {
		if strings.TrimSpace(claim.Text) == "" {
			problems = append(problems, fmt.Sprintf("claim %d is empty", index+1))
		}
		if claim.Status == "unverified" || len(claim.SourceIDs) == 0 {
			verification = "unverified_warning"
		}
		if claim.Status != "unverified" && len(claim.SourceIDs) == 0 {
			problems = append(problems, fmt.Sprintf("claim %d is labelled %s but has no source", index+1, claim.Status))
		}
		if claim.Status == "interpretation" && verification == "verified" {
			verification = "partially_verified"
		}
		for _, sourceID := range claim.SourceIDs {
			if !sourceIDs[sourceID] {
				problems = append(problems, fmt.Sprintf("claim %d cites missing source %s", index+1, sourceID))
				verification = "unverified_warning"
			}
		}
	}
	if len(allowedEvidence) > 0 && len(draft.Claims) == 0 {
		problems = append(problems, "claims are empty despite supplied evidence")
		verification = "unverified_warning"
	}
	if len(draft.Uncertainty) > 0 && verification == "verified" {
		verification = "partially_verified"
	}
	return problems, verification
}

func minimumLessonWords(minutes int) int {
	if minutes == 30 {
		return 3600
	}
	return 1800
}

type lessonSectionRequirement struct {
	Name         string
	Values       []string
	MinimumItems int
	MinimumWords int
}

func validateSectionDepth(draft LessonDraft, minutes int) []string {
	stringMinimums := map[string]int{
		"motivation": 90, "definition": 100, "production_example": 150,
		"security": 45, "reliability": 45, "performance": 45, "cost": 45,
		"present_maturity": 45, "future_direction": 45, "career_relevance": 45,
		"interview_answer": 45,
	}
	if minutes == 30 {
		stringMinimums = map[string]int{
			"motivation": 160, "definition": 180, "production_example": 300,
			"security": 80, "reliability": 80, "performance": 80, "cost": 80,
			"present_maturity": 80, "future_direction": 80, "career_relevance": 80,
			"interview_answer": 80,
		}
	}
	stringSections := []struct {
		Name  string
		Value string
	}{
		{"motivation", draft.Motivation},
		{"definition", draft.Definition},
		{"production_example", draft.ProductionExample},
		{"security", draft.Security},
		{"reliability", draft.Reliability},
		{"performance", draft.Performance},
		{"cost", draft.Cost},
		{"present_maturity", draft.PresentMaturity},
		{"future_direction", draft.FutureDirection},
		{"career_relevance", draft.CareerRelevance},
		{"interview_answer", draft.InterviewAnswer},
	}

	problems := make([]string, 0)
	for _, section := range stringSections {
		if strings.TrimSpace(section.Value) == "" {
			continue
		}
		actual := wordCount(section.Value)
		minimum := stringMinimums[section.Name]
		if actual < minimum {
			problems = append(problems, fmt.Sprintf(
				"%s needs at least %d words for the %d-minute preset; got %d",
				section.Name, minimum, minutes, actual,
			))
		}
	}

	listRequirements := []lessonSectionRequirement{
		{Name: "prior_approaches", Values: draft.PriorApproaches, MinimumItems: 2, MinimumWords: 120},
		{Name: "mechanics", Values: draft.Mechanics, MinimumItems: 4, MinimumWords: 300},
		{Name: "tradeoffs", Values: draft.Tradeoffs, MinimumItems: 3, MinimumWords: 160},
		{Name: "failure_modes", Values: draft.FailureModes, MinimumItems: 3, MinimumWords: 160},
		{Name: "when_not_to_use", Values: draft.WhenNotToUse, MinimumItems: 2, MinimumWords: 100},
		{Name: "alternatives", Values: draft.Alternatives, MinimumItems: 2, MinimumWords: 100},
	}
	if minutes == 30 {
		listRequirements = []lessonSectionRequirement{
			{Name: "prior_approaches", Values: draft.PriorApproaches, MinimumItems: 2, MinimumWords: 220},
			{Name: "mechanics", Values: draft.Mechanics, MinimumItems: 4, MinimumWords: 600},
			{Name: "tradeoffs", Values: draft.Tradeoffs, MinimumItems: 3, MinimumWords: 300},
			{Name: "failure_modes", Values: draft.FailureModes, MinimumItems: 3, MinimumWords: 300},
			{Name: "when_not_to_use", Values: draft.WhenNotToUse, MinimumItems: 2, MinimumWords: 180},
			{Name: "alternatives", Values: draft.Alternatives, MinimumItems: 2, MinimumWords: 180},
		}
	}
	for _, requirement := range listRequirements {
		if len(requirement.Values) == 0 {
			continue
		}
		if len(requirement.Values) < requirement.MinimumItems {
			problems = append(problems, fmt.Sprintf(
				"%s needs at least %d developed points for the %d-minute preset; got %d",
				requirement.Name, requirement.MinimumItems, minutes, len(requirement.Values),
			))
		}
		actual := wordCount(requirement.Values...)
		if actual < requirement.MinimumWords {
			problems = append(problems, fmt.Sprintf(
				"%s needs at least %d words across its points for the %d-minute preset; got %d",
				requirement.Name, requirement.MinimumWords, minutes, actual,
			))
		}
	}
	return problems
}

func lessonWordCount(draft LessonDraft) int {
	values := []string{draft.Title, draft.Motivation, draft.Definition, draft.ProductionExample,
		draft.Security, draft.Reliability, draft.Performance, draft.Cost, draft.PresentMaturity,
		draft.FutureDirection, draft.CareerRelevance, draft.InterviewAnswer, draft.RecallQuestion}
	values = append(values, draft.PriorApproaches...)
	values = append(values, draft.Mechanics...)
	values = append(values, draft.Tradeoffs...)
	values = append(values, draft.FailureModes...)
	values = append(values, draft.WhenNotToUse...)
	values = append(values, draft.Alternatives...)
	return wordCount(values...)
}

func wordCount(values ...string) int {
	count := 0
	for _, value := range values {
		count += len(strings.Fields(value))
	}
	return count
}

func Usable(draft LessonDraft) bool {
	return strings.TrimSpace(draft.Title) != "" && strings.TrimSpace(draft.Motivation) != "" &&
		strings.TrimSpace(draft.Definition) != "" && len(draft.Mechanics) > 0
}
