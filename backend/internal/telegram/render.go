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
			chunks = append(chunks, splitHTML(paragraph, limit)...)
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

type openHTMLTag struct {
	name  string
	value string
}

func splitHTML(value string, limit int) []string {
	tokens := htmlTokens(value)
	active := make([]openHTMLTag, 0, 3)
	var chunks []string
	var current strings.Builder

	startChunk := func() {
		for _, tag := range active {
			current.WriteString(tag.value)
		}
	}
	closeTags := func() string {
		var closing strings.Builder
		for index := len(active) - 1; index >= 0; index-- {
			closing.WriteString("</" + active[index].name + ">")
		}
		return closing.String()
	}
	flush := func() {
		if current.Len() == 0 {
			return
		}
		chunk := strings.TrimSpace(current.String() + closeTags())
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		current.Reset()
		startChunk()
	}

	for _, token := range tokens {
		if token.openName != "" {
			closing := closeTags() + "</" + token.openName + ">"
			if current.Len() > 0 && runeCount(current.String()+token.value+closing) > limit {
				flush()
			}
			current.WriteString(token.value)
			active = append(active, openHTMLTag{name: token.openName, value: token.value})
			continue
		}
		if token.closeName != "" {
			current.WriteString(token.value)
			for index := len(active) - 1; index >= 0; index-- {
				if active[index].name == token.closeName {
					active = active[:index]
					break
				}
			}
			continue
		}

		for _, segment := range splitTextToken(token.value) {
			if current.Len() > 0 && runeCount(current.String()+segment+closeTags()) > limit {
				flush()
			}
			if runeCount(current.String()+segment+closeTags()) <= limit {
				current.WriteString(segment)
				continue
			}
			for _, character := range []rune(segment) {
				if current.Len() > 0 && runeCount(current.String()+string(character)+closeTags()) > limit {
					flush()
				}
				current.WriteRune(character)
			}
		}
	}
	if current.Len() > 0 {
		chunk := strings.TrimSpace(current.String())
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
	}
	return chunks
}

type htmlToken struct {
	value     string
	openName  string
	closeName string
}

func htmlTokens(value string) []htmlToken {
	var tokens []htmlToken
	for len(value) > 0 {
		if value[0] == '<' {
			if end := strings.IndexByte(value, '>'); end >= 0 {
				raw := value[:end+1]
				name, closing := htmlTagName(raw)
				token := htmlToken{value: raw}
				if closing {
					token.closeName = name
				} else {
					token.openName = name
				}
				tokens = append(tokens, token)
				value = value[end+1:]
				continue
			}
		}
		if value[0] == '&' {
			if end := strings.IndexByte(value, ';'); end >= 1 && end <= 16 {
				tokens = append(tokens, htmlToken{value: value[:end+1]})
				value = value[end+1:]
				continue
			}
		}
		next := len(value)
		if index := strings.IndexAny(value[1:], "<&"); index >= 0 {
			next = index + 1
		}
		tokens = append(tokens, htmlToken{value: value[:next]})
		value = value[next:]
	}
	return tokens
}

func htmlTagName(raw string) (string, bool) {
	value := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(raw, "<"), ">"))
	closing := strings.HasPrefix(value, "/")
	value = strings.TrimPrefix(value, "/")
	if separator := strings.IndexAny(value, " \t\r\n"); separator >= 0 {
		value = value[:separator]
	}
	return strings.ToLower(value), closing
}

func splitTextToken(value string) []string {
	var segments []string
	var current strings.Builder
	for _, character := range value {
		current.WriteRune(character)
		if character == ' ' || character == '\n' || character == '\t' {
			segments = append(segments, current.String())
			current.Reset()
		}
	}
	if current.Len() > 0 {
		segments = append(segments, current.String())
	}
	return segments
}

func runeCount(value string) int { return utf8.RuneCountInString(value) }
