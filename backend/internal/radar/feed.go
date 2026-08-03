package radar

import (
	"encoding/xml"
	"errors"
	"html"
	"regexp"
	"strings"
	"time"
)

type Item struct {
	Title, URL, Summary string
	PublishedAt         time.Time
}

type feed struct {
	Channel struct {
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
	Entries []atomEntry `xml:"entry"`
}
type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	Published   string `xml:"pubDate"`
}
type atomEntry struct {
	Title string `xml:"title"`
	Links []struct {
		Href string `xml:"href,attr"`
		Rel  string `xml:"rel,attr"`
	} `xml:"link"`
	Summary   string `xml:"summary"`
	Content   string `xml:"content"`
	Published string `xml:"published"`
	Updated   string `xml:"updated"`
}

var tags = regexp.MustCompile(`<[^>]+>`)

func ParseFeed(input []byte) ([]Item, error) {
	var document feed
	if err := xml.Unmarshal(input, &document); err != nil {
		return nil, err
	}
	var items []Item
	for _, entry := range document.Channel.Items {
		published := parseTime(entry.Published)
		items = append(items, Item{Title: clean(entry.Title), URL: strings.TrimSpace(entry.Link), Summary: clean(entry.Description), PublishedAt: published})
	}
	for _, entry := range document.Entries {
		link := ""
		for _, candidate := range entry.Links {
			if candidate.Rel == "" || candidate.Rel == "alternate" {
				link = candidate.Href
				break
			}
		}
		summary := entry.Summary
		if summary == "" {
			summary = entry.Content
		}
		published := parseTime(entry.Published)
		if published.IsZero() {
			published = parseTime(entry.Updated)
		}
		items = append(items, Item{Title: clean(entry.Title), URL: strings.TrimSpace(link), Summary: clean(summary), PublishedAt: published})
	}
	if len(items) == 0 {
		return nil, errors.New("feed contains no items")
	}
	return items, nil
}

func clean(value string) string {
	value = html.UnescapeString(tags.ReplaceAllString(value, " "))
	return strings.Join(strings.Fields(value), " ")
}
func parseTime(value string) time.Time {
	for _, layout := range []string{time.RFC1123Z, time.RFC1123, time.RFC3339, time.RFC822Z, time.RFC822} {
		if parsed, err := time.Parse(layout, strings.TrimSpace(value)); err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}
