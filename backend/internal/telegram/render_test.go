package telegram

import (
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
