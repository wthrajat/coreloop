package content

import (
	"strings"
	"testing"
	"time"
)

func TestGroundSourcesRemovesModelOnlyPlaceholderURL(t *testing.T) {
	draft := LessonDraft{Sources: []Source{{
		ID:           "invented",
		Publisher:    "Example",
		Title:        "Placeholder",
		CanonicalURL: "https://example.com/source",
	}}}

	grounded, problems := GroundSources(draft, nil)

	if len(grounded.Sources) != 0 {
		t.Fatalf("unexpected ungrounded sources: %#v", grounded.Sources)
	}
	if len(problems) != 1 || !strings.Contains(problems[0], "was not supplied") {
		t.Fatalf("unexpected grounding problems: %v", problems)
	}
	if sections := RenderSections(grounded); containsString(sections, "example.com") {
		t.Fatalf("placeholder URL reached rendering: %v", sections)
	}
}

func TestGroundSourcesUsesExactEvidenceMetadata(t *testing.T) {
	publishedAt := time.Date(2026, time.August, 5, 10, 0, 0, 0, time.UTC)
	draft := LessonDraft{Sources: []Source{{
		ID:           "evidence-1",
		Publisher:    "Invented publisher",
		Title:        "Invented title",
		CanonicalURL: "https://example.com/invented",
	}}}
	evidence := Evidence{
		ID:           "evidence-1",
		Publisher:    "The Go Blog",
		Title:        "A real article",
		CanonicalURL: "https://go.dev/blog/example",
		PublishedAt:  publishedAt,
	}

	grounded, problems := GroundSources(draft, []Evidence{evidence})

	if len(grounded.Sources) != 1 {
		t.Fatalf("grounded sources: %#v", grounded.Sources)
	}
	got := grounded.Sources[0]
	if got.Publisher != evidence.Publisher || got.Title != evidence.Title ||
		got.CanonicalURL != evidence.CanonicalURL || got.PublishedAt != publishedAt.Format(time.RFC3339) {
		t.Fatalf("source metadata was not grounded: %#v", got)
	}
	if len(problems) != 1 || !strings.Contains(problems[0], "does not match") {
		t.Fatalf("expected spoofed URL to request correction: %v", problems)
	}
}

func TestDeliverableSourceURLRejectsPlaceholderAndUnsafeURLs(t *testing.T) {
	for _, rawURL := range []string{
		"https://example.com/source",
		"https://docs.example/source",
		"http://go.dev/blog/source",
		"https://user:secret@go.dev/blog/source",
		"javascript:alert(1)",
	} {
		if DeliverableSourceURL(rawURL) {
			t.Fatalf("expected %q to be rejected", rawURL)
		}
	}
	if !DeliverableSourceURL("https://go.dev/blog/source") {
		t.Fatal("expected official HTTPS URL to be deliverable")
	}
}

func TestSanitizeRenderedSourceLinksProtectsPersistedLessonParts(t *testing.T) {
	rendered := `<b>Sources</b>
<a href="https://example.com/invented">Example — Invented</a>
<a href="https://go.dev/blog/source?id=1&amp;lang=en">Go — Source</a>`

	sanitized := SanitizeRenderedSourceLinks(rendered)

	if strings.Contains(sanitized, `href="https://example.com`) ||
		!strings.Contains(sanitized, "Example — Invented (source link unavailable)") {
		t.Fatalf("placeholder link was not neutralized: %s", sanitized)
	}
	if !strings.Contains(sanitized, `href="https://go.dev/blog/source?id=1&amp;lang=en"`) {
		t.Fatalf("valid source link was removed: %s", sanitized)
	}
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
