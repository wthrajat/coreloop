package content

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

const StableInstructions = `You write rigorous technical lessons for working software engineers.
Teach in plain English while preserving precise professional terminology. Technical analogies are allowed; childish analogies are not.
Information and usefulness are more important than decorative structure. Start with why the topic exists, the problem it solves, and what came before it. Then explain definition, mechanics, a realistic production scenario, trade-offs, failure modes, when not to use it, alternatives, security, reliability, performance, cost, current maturity, future direction, career relevance, and an interview-ready explanation.
Every lesson is detailed, including a 15-minute preset. Keep it mostly theoretical; use code only when it materially clarifies a concept. English only.
Use only supplied evidence for time-sensitive claims. Cite each factual claim by source id. Mark interpretation as interpretation and list uncertainty honestly. Do not invent sources, URLs, dates, benchmarks, adoption, or product behavior.
Return only JSON matching the supplied schema. All fields are required. Do not add markdown fences or commentary.`

func Compile(context LessonContext, correctionErrors []string) (string, string, error) {
	dynamic := struct {
		LessonContext
		CorrectionErrors []string `json:"fix,omitempty"`
	}{LessonContext: context, CorrectionErrors: correctionErrors}
	encoded, err := json.Marshal(dynamic)
	if err != nil {
		return "", "", fmt.Errorf("encode lesson context: %w", err)
	}
	if EstimateTokens(StableInstructions)+EstimateTokens(string(encoded))+OutputBudget(context.Minutes) > 30_000 {
		return "", "", fmt.Errorf("lesson context exceeds the safe model budget")
	}
	return StableInstructions, string(encoded), nil
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
		TopicID    string   `json:"topic"`
		Level      string   `json:"level"`
		Minutes    int      `json:"minutes"`
		Depth      string   `json:"depth"`
		Objectives []string `json:"objectives"`
		Evidence   []struct {
			ID          string    `json:"id"`
			PublishedAt time.Time `json:"published_at"`
		} `json:"evidence"`
		PromptVersion string `json:"prompt_version"`
		SchemaVersion string `json:"schema_version"`
	}{TopicID: context.TopicID, Level: context.Level, Minutes: context.Minutes, Depth: context.Depth,
		Objectives: context.Objectives, PromptVersion: PromptVersion, SchemaVersion: SchemaVersion}
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
