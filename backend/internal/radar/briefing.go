package radar

import (
	"errors"
	"html"
	"regexp"
	"strings"
	"unicode/utf8"
)

type SourceReference struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type BriefingInput struct {
	Category          Category
	Title             string
	Summary           string
	WhyItMatters      string
	SimpleExplanation string
	Source            SourceReference
	DiscoveredVia     []SourceReference
}

type briefingTextLimits struct {
	Summary     int
	Context     int
	Explanation int
}

var callToAction = regexp.MustCompile(`(?i)^\s*(?:learn more|read more|read the (?:full|official) (?:article|post|source)|click here|sign up|register now|register today|get started|contact sales|try (?:it|this) (?:now|today)|watch now|download (?:it )?today|visit .+ to learn more)\b`)
var announcementBoilerplate = regexp.MustCompile(`(?i)\b(?:we|[a-z0-9&.-]+)\s+(?:are|is)\s+(?:excited|thrilled|proud|delighted)\s+to\s+announce\s+`)
var promotionalLanguage = regexp.MustCompile(`(?i)\b(?:best-in-class|industry-leading|market-leading|world-class|cutting-edge|game-changing|groundbreaking|revolutionary|unmatched|unparalleled|transformative|next-generation|seamlessly|effortlessly)\b`)
var whitespaceBeforePunctuation = regexp.MustCompile(`\s+([,.;:!?])`)

// NeutralText removes calls to action and common promotional boilerplate while
// retaining concrete facts, numbers, product names, and comparisons.
func NeutralText(value string) string {
	value = clean(value)
	if value == "" {
		return ""
	}
	parts := splitSentences(value)
	neutral := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || callToAction.MatchString(part) {
			continue
		}
		part = announcementBoilerplate.ReplaceAllString(part, "")
		part = promotionalLanguage.ReplaceAllString(part, "")
		part = strings.Join(strings.Fields(part), " ")
		part = whitespaceBeforePunctuation.ReplaceAllString(part, "$1")
		part = strings.TrimSpace(part)
		if part != "" {
			neutral = append(neutral, part)
		}
	}
	return strings.Join(neutral, " ")
}

// RenderBriefing produces a detailed plain-text fallback that does not depend
// on an AI provider. Telegram-specific escaping and chunking remain the
// responsibility of the delivery layer.
func RenderBriefing(input BriefingInput) (string, error) {
	sourceName := clean(input.Source.Name)
	if sourceName == "" {
		return "", errors.New("briefing source name is required")
	}
	sourceURL, err := CanonicalURL(input.Source.URL)
	if err != nil {
		return "", err
	}
	category := input.Category
	if category == "" {
		category = Classify(input.Title, input.Summary)
	}
	title := NeutralText(input.Title)
	if title == "" {
		return "", errors.New("briefing title is required")
	}

	sections := []string{category.Label() + " · " + sourceName, title}
	if summary := NeutralText(input.Summary); summary != "" && !strings.EqualFold(summary, title) {
		sections = append(sections, "What happened\n"+summary)
	}
	if context := NeutralText(input.WhyItMatters); context != "" {
		sections = append(sections, "Why it matters\n"+context)
	}
	if explanation := NeutralText(input.SimpleExplanation); explanation != "" {
		sections = append(sections, "Simple explanation\n"+explanation)
	}
	sections = append(sections, "Source\n"+sourceName+"\n"+sourceURL)

	additionalSources := renderAdditionalSources(input.DiscoveredVia, sourceURL)
	if additionalSources != "" {
		sections = append(sections, "Discovered via\n"+additionalSources)
	}
	return strings.Join(sections, "\n\n"), nil
}

// RenderCompactBriefing preserves one source-backed news item inside one
// delivery message. Release notices are intentionally terse; incidents,
// research, and engineering articles retain more useful evidence.
func RenderCompactBriefing(input BriefingInput, maximumEscapedRunes int) (string, error) {
	if maximumEscapedRunes <= 0 {
		return "", errors.New("briefing character limit must be positive")
	}
	category := input.Category
	if category == "" {
		category = Classify(input.Title, input.Summary)
	}
	limits := compactBriefingLimits(category)
	input.Summary = truncateNeutralText(input.Summary, limits.Summary)
	input.WhyItMatters = truncateNeutralText(input.WhyItMatters, limits.Context)
	input.SimpleExplanation = truncateNeutralText(
		input.SimpleExplanation,
		limits.Explanation,
	)

	briefing, err := RenderBriefing(input)
	if err != nil {
		return "", err
	}
	if escapedRuneCount(briefing) <= maximumEscapedRunes {
		return briefing, nil
	}

	// Scale both optional sections down together. The required category,
	// headline, publisher, and source URL always remain intact.
	originalSummary := input.Summary
	originalContext := input.WhyItMatters
	originalExplanation := input.SimpleExplanation
	bestInput := input
	bestInput.Summary = ""
	bestInput.WhyItMatters = ""
	bestInput.SimpleExplanation = ""
	best, err := RenderBriefing(bestInput)
	if err != nil {
		return "", err
	}
	if escapedRuneCount(best) > maximumEscapedRunes {
		// Discovery links are useful provenance but secondary to the original
		// source. Drop them before sacrificing the explanatory sections.
		input.DiscoveredVia = nil
		bestInput.DiscoveredVia = nil
		best, err = RenderBriefing(bestInput)
		if err != nil {
			return "", err
		}
		if escapedRuneCount(best) > maximumEscapedRunes {
			return "", errors.New("required briefing fields exceed character limit")
		}
	}
	low, high := 0.0, 1.0
	for range 18 {
		scale := (low + high) / 2
		candidateInput := input
		candidateInput.Summary = truncateNeutralText(
			originalSummary, int(float64(utf8.RuneCountInString(originalSummary))*scale),
		)
		candidateInput.WhyItMatters = truncateNeutralText(
			originalContext, int(float64(utf8.RuneCountInString(originalContext))*scale),
		)
		candidateInput.SimpleExplanation = truncateNeutralText(
			originalExplanation,
			int(float64(utf8.RuneCountInString(originalExplanation))*scale),
		)
		candidate, renderErr := RenderBriefing(candidateInput)
		if renderErr != nil {
			return "", renderErr
		}
		if escapedRuneCount(candidate) <= maximumEscapedRunes {
			best, low = candidate, scale
		} else {
			high = scale
		}
	}
	return best, nil
}

func compactBriefingLimits(category Category) briefingTextLimits {
	switch category {
	case CategoryRelease:
		return briefingTextLimits{Summary: 600, Context: 220, Explanation: 360}
	case CategorySecurity, CategoryIndustry:
		return briefingTextLimits{Summary: 1_500, Context: 450, Explanation: 700}
	case CategoryEngineering, CategoryResearch, CategoryDiscussion:
		return briefingTextLimits{Summary: 1_400, Context: 450, Explanation: 700}
	default:
		return briefingTextLimits{Summary: 800, Context: 280, Explanation: 500}
	}
}

func truncateNeutralText(value string, maximumRunes int) string {
	value = NeutralText(value)
	if value == "" || maximumRunes <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= maximumRunes {
		return value
	}
	if maximumRunes == 1 {
		return "…"
	}
	cut := maximumRunes - 1
	for index := cut; index > cut/2; index-- {
		if runes[index-1] == ' ' || runes[index-1] == '\n' {
			cut = index - 1
			break
		}
	}
	return strings.TrimSpace(string(runes[:cut])) + "…"
}

func escapedRuneCount(value string) int {
	return utf8.RuneCountInString(html.EscapeString(value))
}

func renderAdditionalSources(sources []SourceReference, primarySourceURL string) string {
	lines := make([]string, 0, len(sources))
	seen := map[string]bool{primarySourceURL: true}
	for _, source := range sources {
		name := clean(source.Name)
		canonical, err := CanonicalURL(source.URL)
		if name == "" || err != nil || seen[canonical] {
			continue
		}
		seen[canonical] = true
		lines = append(lines, name+": "+canonical)
	}
	return strings.Join(lines, "\n")
}

func splitSentences(value string) []string {
	parts := make([]string, 0, 4)
	start := 0
	for index := 0; index < len(value); index++ {
		if value[index] != '.' && value[index] != '!' && value[index] != '?' {
			continue
		}
		end := index + 1
		for end < len(value) && (value[end] == '.' || value[end] == '!' || value[end] == '?') {
			end++
		}
		if end < len(value) && value[end] != ' ' && value[end] != '\n' && value[end] != '\t' {
			index = end - 1
			continue
		}
		parts = append(parts, value[start:end])
		for end < len(value) && (value[end] == ' ' || value[end] == '\n' || value[end] == '\t') {
			end++
		}
		start = end
		index = end - 1
	}
	if start < len(value) {
		parts = append(parts, value[start:])
	}
	return parts
}
