package content

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

const StableInstructions = `You write rigorous technical lessons for working software engineers.
Use simple, precise English. Prefer short direct sentences, common words, and one main idea per paragraph. Keep necessary professional terminology, but briefly define each unfamiliar technical term or acronym at first use in parentheses, for example: lease (a temporary right to use a resource). Do not repeat the definition later. Avoid unexplained jargon, dense noun phrases, and childish analogies.
Information and usefulness are more important than decorative structure. Start with why the topic exists, the problem it solves, and what came before it. Then explain definition, mechanics, a realistic production scenario, trade-offs, failure modes, when not to use it, alternatives, security, reliability, performance, cost, current maturity, future direction, career relevance, and an interview-ready explanation.
Explain each selected objective in two layers: first the simple mental model, then the precise mechanics and consequences. Required sections must contain distinct explanation, not one-sentence placeholders. Do not pad the lesson by repeating an idea in different words.
Use position, prerequisites, and covered_objectives to continue the theme. Build on earlier coverage. Revisit an earlier objective only to deepen, connect, or apply it; never repeat the earlier lesson.
Follow generation_requirements exactly. Every lesson is detailed, including a 15-minute preset. Keep it mostly theoretical; use code only when it materially clarifies a concept. English only.
Use only supplied evidence for time-sensitive claims. Cite each factual claim by source id. Copy source ids and metadata exactly from supplied evidence; if evidence is empty, return an empty sources array. Mark interpretation as interpretation and list uncertainty honestly. Do not invent sources, URLs, dates, benchmarks, adoption, or product behavior.
If fix is present, revise the whole lesson and repair every listed issue without shortening sections that were already useful.
Return only JSON matching the supplied schema. All fields are required. Do not add markdown fences or commentary.`

type generationRequirements struct {
	WordTarget       string `json:"word_target"`
	SectionDepth     string `json:"section_depth"`
	ExplanationDepth string `json:"explanation_depth"`
}

func Compile(context LessonContext, correctionErrors []string) (string, string, error) {
	dynamic := struct {
		LessonContext
		GenerationRequirements generationRequirements `json:"generation_requirements"`
		CorrectionErrors       []string               `json:"fix,omitempty"`
	}{
		LessonContext:          context,
		GenerationRequirements: requirementsFor(context),
		CorrectionErrors:       correctionErrors,
	}
	encoded, err := json.Marshal(dynamic)
	if err != nil {
		return "", "", fmt.Errorf("encode lesson context: %w", err)
	}
	if EstimateTokens(StableInstructions)+EstimateTokens(string(encoded))+OutputBudget(context.Minutes) > 30_000 {
		return "", "", fmt.Errorf("lesson context exceeds the safe model budget")
	}
	return StableInstructions, string(encoded), nil
}

func requirementsFor(context LessonContext) generationRequirements {
	requirements := generationRequirements{
		WordTarget: "1,900-2,400 useful words; never trade clarity for length",
		SectionDepth: "Develop the mental model, mechanics, and production scenario fully. " +
			"Use several concrete mechanics, trade-offs, failure modes, and alternatives.",
	}
	if context.Minutes >= 30 {
		requirements.WordTarget = "3,800-4,500 useful words; add depth and examples, not repetition"
		requirements.SectionDepth = "Treat this as a genuinely deep lesson. Explain causal chains step by step, " +
			"show how components interact in production, compare realistic design choices, and trace at least one failure from cause through detection, impact, and recovery."
	}
	switch context.Depth {
	case "foundation":
		requirements.ExplanationDepth = "Build prerequisites and vocabulary before internals. Assume the learner may know adjacent engineering concepts but not this topic."
	case "detailed":
		requirements.ExplanationDepth = "Go beyond definitions into internals, operational behavior, edge cases, and decision criteria while keeping the language easy to follow."
	default:
		requirements.ExplanationDepth = "Explain the core internals and the production decisions a working engineer should understand."
	}
	return requirements
}

func EstimateTokens(value string) int { return (len([]rune(value)) + 2) / 3 }

func OutputBudget(minutes int) int {
	if minutes >= 30 {
		return 8_000
	}
	return 4_500
}

func PromptChecksum() string {
	sum := sha256.Sum256([]byte(StableInstructions))
	return hex.EncodeToString(sum[:])
}

func CacheKey(context LessonContext) (string, error) {
	value := struct {
		TopicID           string   `json:"topic"`
		Level             string   `json:"level"`
		Minutes           int      `json:"minutes"`
		Depth             string   `json:"depth"`
		Position          int      `json:"position"`
		Objectives        []string `json:"objectives"`
		Prerequisites     []string `json:"prerequisites"`
		CoveredObjectives []string `json:"covered_objectives"`
		Evidence          []struct {
			ID          string    `json:"id"`
			PublishedAt time.Time `json:"published_at"`
		} `json:"evidence"`
		PromptVersion string `json:"prompt_version"`
		SchemaVersion string `json:"schema_version"`
	}{
		TopicID: context.TopicID, Level: context.Level, Minutes: context.Minutes,
		Depth: context.Depth, Position: context.Position, Objectives: context.Objectives,
		Prerequisites: context.Prerequisites, CoveredObjectives: context.CoveredObjectives,
		PromptVersion: PromptVersion, SchemaVersion: SchemaVersion,
	}
	for _, evidence := range context.Evidence {
		value.Evidence = append(value.Evidence, struct {
			ID          string    `json:"id"`
			PublishedAt time.Time `json:"published_at"`
		}{evidence.ID, evidence.PublishedAt})
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}
