package content

import (
	"fmt"
	"html"
	"strings"
)

func RenderSections(draft LessonDraft) []string {
	sections := []string{
		"<b>" + escape(draft.Title) + "</b>\n<i>Estimated reading time: " + fmt.Sprint(draft.EstimatedMinutes) + " minutes</i>",
	}
	for _, rendered := range []string{
		section("Why this matters", draft.Motivation),
		listSection("What existed before", draft.PriorApproaches),
		section("What it is", draft.Definition),
		listSection("How it works", draft.Mechanics),
		section("Production scenario", draft.ProductionExample),
		listSection("Trade-offs", draft.Tradeoffs),
		listSection("Failure modes", draft.FailureModes),
		listSection("When not to use it", draft.WhenNotToUse),
		listSection("Alternatives", draft.Alternatives),
		section("Security", draft.Security),
		section("Reliability", draft.Reliability),
		section("Performance", draft.Performance),
		section("Cost", draft.Cost),
		section("Where it stands today", draft.PresentMaturity),
		section("Where it may go", draft.FutureDirection),
		section("Career relevance", draft.CareerRelevance),
		section("Interview answer", draft.InterviewAnswer),
		section("Recall question", draft.RecallQuestion),
	} {
		if rendered != "" {
			sections = append(sections, rendered)
		}
	}
	if len(draft.Uncertainty) > 0 {
		sections = append(sections, listSection("Uncertainty", draft.Uncertainty))
	}
	if len(draft.Sources) > 0 {
		var sources []string
		for _, source := range draft.Sources {
			if !DeliverableSourceURL(source.CanonicalURL) {
				continue
			}
			sources = append(sources, fmt.Sprintf(`<a href="%s">%s — %s</a>`, html.EscapeString(source.CanonicalURL), escape(source.Publisher), escape(source.Title)))
		}
		if len(sources) > 0 {
			sections = append(sections, "<b>Sources</b>\n"+strings.Join(sources, "\n"))
		}
	}
	return sections
}

func section(title, body string) string {
	body = escape(body)
	if body == "" {
		return ""
	}
	return "<b>" + escape(title) + "</b>\n" + body
}
func listSection(title string, values []string) string {
	var items []string
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			items = append(items, "• "+escape(value))
		}
	}
	if len(items) == 0 {
		return ""
	}
	return "<b>" + escape(title) + "</b>\n" + strings.Join(items, "\n")
}
func escape(value string) string { return html.EscapeString(strings.TrimSpace(value)) }
