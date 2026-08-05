package content

import (
	"html"
	"net/url"
	"regexp"
	"strings"
	"time"

	"coreloop/backend/internal/urlguard"
)

var renderedSourceLinkPattern = regexp.MustCompile(
	`(?i)<a href="([^"]+)">([^<]*)</a>`,
)

// GroundSources replaces model-authored source metadata with the exact
// backend-supplied evidence. Unknown or unsafe sources are removed before a
// usable fallback response can be rendered or cached.
func GroundSources(
	draft LessonDraft,
	allowedEvidence []Evidence,
) (LessonDraft, []string) {
	allowed := make(map[string]Evidence, len(allowedEvidence))
	for _, evidence := range allowedEvidence {
		if strings.TrimSpace(evidence.ID) != "" {
			allowed[evidence.ID] = evidence
		}
	}

	grounded := make([]Source, 0, len(draft.Sources))
	seen := make(map[string]bool, len(draft.Sources))
	var problems []string
	for _, source := range draft.Sources {
		evidence, exists := allowed[source.ID]
		if !exists {
			problems = append(problems, "source "+source.ID+" was not supplied")
			continue
		}
		if seen[source.ID] {
			continue
		}
		if !DeliverableSourceURL(evidence.CanonicalURL) {
			problems = append(problems, "source "+source.ID+" has an invalid evidence URL")
			continue
		}
		if strings.TrimSpace(source.CanonicalURL) != strings.TrimSpace(evidence.CanonicalURL) {
			problems = append(problems, "source "+source.ID+" URL does not match supplied evidence")
		}
		grounded = append(grounded, sourceFromEvidence(evidence))
		seen[source.ID] = true
	}
	draft.Sources = grounded
	return draft, problems
}

func sourceFromEvidence(evidence Evidence) Source {
	publishedAt := ""
	if !evidence.PublishedAt.IsZero() {
		publishedAt = evidence.PublishedAt.UTC().Format(time.RFC3339)
	}
	return Source{
		ID:           evidence.ID,
		Publisher:    evidence.Publisher,
		Title:        evidence.Title,
		CanonicalURL: evidence.CanonicalURL,
		PublishedAt:  publishedAt,
	}
}

func DeliverableSourceURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return false
	}
	return !urlguard.IsPlaceholderHost(parsed.Hostname())
}

// SanitizeRenderedSourceLinks protects delivery of already-persisted lesson
// parts created before source grounding was enforced. It keeps the source label
// readable while removing unsafe or placeholder links.
func SanitizeRenderedSourceLinks(rendered string) string {
	return renderedSourceLinkPattern.ReplaceAllStringFunc(rendered, func(link string) string {
		matches := renderedSourceLinkPattern.FindStringSubmatch(link)
		if len(matches) != 3 || DeliverableSourceURL(html.UnescapeString(matches[1])) {
			return link
		}
		return matches[2] + " (source link unavailable)"
	})
}
