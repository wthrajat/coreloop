package radar

import (
	"strings"
	"testing"
)

func TestNeutralTextRemovesMarketingButKeepsFacts(t *testing.T) {
	input := "CloudCo is excited to announce Database X 1.2.3, a revolutionary, industry-leading database. It supports PostgreSQL and provides up to 65% lower latency in three regions. Learn more today."
	got := NeutralText(input)
	for _, unwanted := range []string{"excited", "revolutionary", "industry-leading", "Learn more"} {
		if strings.Contains(strings.ToLower(got), strings.ToLower(unwanted)) {
			t.Fatalf("NeutralText() retained %q in %q", unwanted, got)
		}
	}
	for _, fact := range []string{"Database X 1.2.3", "PostgreSQL", "65% lower latency", "three regions"} {
		if !strings.Contains(got, fact) {
			t.Fatalf("NeutralText() removed fact %q from %q", fact, got)
		}
	}
}

func TestRenderBriefingIncludesDetailedFactsAndEveryValidSource(t *testing.T) {
	got, err := RenderBriefing(BriefingInput{
		Category:     CategoryProductUpdate,
		Title:        "EC2 I8g instances now available in Paris and Jakarta",
		Summary:      "I8g instances use Graviton4 processors and local Nitro NVMe storage. AWS reports up to 65% better real-time storage performance per TB than I4g.",
		WhyItMatters: "This affects teams evaluating storage-intensive workloads in these regions.",
		Source:       SourceReference{Name: "AWS What's New", URL: "http://aws.amazon.com/about-aws/whats-new/example?utm_source=rss#top"},
		DiscoveredVia: []SourceReference{
			{Name: "Hacker News", URL: "https://news.ycombinator.com/item?id=123"},
			{Name: "Duplicate AWS", URL: "https://aws.amazon.com/about-aws/whats-new/example"},
			{Name: "Broken", URL: "not a valid URL %"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"Product update · AWS What's New", "What happened", "Graviton4 processors",
		"65% better real-time storage performance", "Why it matters",
		"https://aws.amazon.com/about-aws/whats-new/example",
		"Hacker News: https://news.ycombinator.com/item?id=123",
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("briefing missing %q:\n%s", expected, got)
		}
	}
	if strings.Contains(got, "Broken") || strings.Contains(got, "Duplicate AWS") || strings.Contains(got, "utm_source") {
		t.Fatalf("briefing retained an invalid or tracking source:\n%s", got)
	}
}

func TestRenderBriefingRequiresOriginalSource(t *testing.T) {
	_, err := RenderBriefing(BriefingInput{Title: "News", Source: SourceReference{Name: "Example"}})
	if err == nil {
		t.Fatal("RenderBriefing() unexpectedly accepted a missing source URL")
	}
}
