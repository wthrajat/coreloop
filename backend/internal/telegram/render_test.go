package telegram

import (
	"html"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestChunkHTMLRespectsTelegramLimit(t *testing.T) {
	chunks := ChunkHTML([]string{strings.Repeat("technical detail ", 700)}, "")
	if len(chunks) < 2 {
		t.Fatal("expected multiple chunks")
	}
	for _, chunk := range chunks {
		if utf8.RuneCountInString(chunk) > 4096 {
			t.Fatalf("chunk too large: %d", utf8.RuneCountInString(chunk))
		}
	}
}

func TestChunkHTMLPreservesTagsAndEntitiesAcrossSplits(t *testing.T) {
	longText := html.EscapeString(strings.Repeat("systems & reliability ", 500))
	section := `<b>Failure modes</b>\n<a href="https://go.dev/blog/source?id=1&amp;lang=en">` + longText + `</a>`
	chunks := ChunkHTML([]string{section}, "")
	if len(chunks) < 2 {
		t.Fatal("expected oversized HTML to be split")
	}
	for _, chunk := range chunks {
		if utf8.RuneCountInString(chunk) > 4096 {
			t.Fatalf("chunk too large: %d", utf8.RuneCountInString(chunk))
		}
		if strings.Count(chunk, `<a href=`) != strings.Count(chunk, `</a>`) {
			t.Fatalf("anchor was split into invalid HTML: %q", chunk)
		}
		if strings.Contains(chunk, "&am") && !strings.Contains(chunk, "&amp;") {
			t.Fatalf("entity was split: %q", chunk)
		}
	}
}
