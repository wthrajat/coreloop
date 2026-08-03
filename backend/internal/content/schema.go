package content

func JSONSchema() map[string]any {
	stringValue := map[string]any{"type": "string"}
	stringList := map[string]any{"type": "array", "items": stringValue}
	properties := map[string]any{
		"title":              stringValue,
		"estimated_minutes":  map[string]any{"type": "integer", "minimum": 1, "maximum": 60},
		"motivation":         stringValue,
		"prior_approaches":   stringList,
		"definition":         stringValue,
		"mechanics":          stringList,
		"production_example": stringValue,
		"tradeoffs":          stringList,
		"failure_modes":      stringList,
		"when_not_to_use":    stringList,
		"alternatives":       stringList,
		"security":           stringValue,
		"reliability":        stringValue,
		"performance":        stringValue,
		"cost":               stringValue,
		"present_maturity":   stringValue,
		"future_direction":   stringValue,
		"career_relevance":   stringValue,
		"interview_answer":   stringValue,
		"recall_question":    stringValue,
		"claims": map[string]any{"type": "array", "items": map[string]any{
			"type": "object", "additionalProperties": false,
			"properties": map[string]any{
				"text":       stringValue,
				"source_ids": stringList,
				"status":     map[string]any{"type": "string", "enum": []string{"verified", "interpretation", "unverified"}},
			},
			"required": []string{"text", "source_ids", "status"},
		}},
		"sources": map[string]any{"type": "array", "items": map[string]any{
			"type": "object", "additionalProperties": false,
			"properties": map[string]any{
				"id":           stringValue,
				"publisher":    stringValue,
				"title":        stringValue,
				"url":          stringValue,
				"published_at": stringValue,
			},
			"required": []string{"id", "publisher", "title", "url", "published_at"},
		}},
		"uncertainty": stringList,
	}
	required := make([]string, 0, len(properties))
	for _, name := range []string{"title", "estimated_minutes", "motivation", "prior_approaches", "definition",
		"mechanics", "production_example", "tradeoffs", "failure_modes", "when_not_to_use", "alternatives",
		"security", "reliability", "performance", "cost", "present_maturity", "future_direction",
		"career_relevance", "interview_answer", "recall_question", "claims", "sources", "uncertainty"} {
		required = append(required, name)
	}
	return map[string]any{"type": "object", "additionalProperties": false, "properties": properties, "required": required}
}
