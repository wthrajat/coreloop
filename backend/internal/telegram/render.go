package telegram

import (
	"fmt"
	"html"
	"strings"
	"unicode/utf8"
)

const SafeChunkCharacters = 3900

func ChunkHTML(sections []string, warning string) []string {
	var chunks []string
	if warning != "" {
		sections = append([]string{"<b>Verification note</b>\n" + html.EscapeString(warning)}, sections...)
	}
	for _, section := range sections {
		section = strings.TrimSpace(section)
		if section == "" {
			continue
		}
		chunks = append(chunks, splitSection(section, SafeChunkCharacters)...)
	}
	for index := range chunks {
		prefix := fmt.Sprintf("<b>Part %d/%d</b>\n\n", index+1, len(chunks))
		if utf8.RuneCountInString(prefix+chunks[index]) <= maxMessageCharacters {
			chunks[index] = prefix + chunks[index]
		}
	}
	return chunks
}

func splitSection(section string, limit int) []string {
	if utf8.RuneCountInString(section) <= limit {
		return []string{section}
	}
	paragraphs := strings.Split(section, "\n\n")
	var chunks []string
	var current strings.Builder
	flush := func() {
		if current.Len() > 0 {
			chunks = append(chunks, strings.TrimSpace(current.String()))
			current.Reset()
		}
	}
	for _, paragraph := range paragraphs {
		if utf8.RuneCountInString(paragraph) > limit {
			flush()
			runes := []rune(paragraph)
			for len(runes) > 0 {
				size := safeBoundary(runes, limit)
				chunks = append(chunks, strings.TrimSpace(string(runes[:size])))
				runes = runes[size:]
			}
			continue
		}
		separator := ""
		if current.Len() > 0 {
			separator = "\n\n"
		}
		if utf8.RuneCountInString(current.String()+separator+paragraph) > limit {
			flush()
			separator = ""
		}
		current.WriteString(separator)
		current.WriteString(paragraph)
	}
	flush()
	return chunks
}

func safeBoundary(runes []rune, limit int) int {
	if len(runes) <= limit {
		return len(runes)
	}
	for index := limit; index > limit/2; index-- {
		if runes[index-1] == ' ' || runes[index-1] == '\n' || runes[index-1] == '\t' {
			return index
		}
	}
	return limit
}
