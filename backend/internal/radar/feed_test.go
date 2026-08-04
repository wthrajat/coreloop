package radar

import "testing"

func TestParseAtomFeed(t *testing.T) {
	items, err := ParseFeed([]byte(`<feed><entry><title>Go 1.30</title><link href="https://go.dev/x"/><summary>Runtime changes</summary><updated>2026-08-03T00:00:00Z</updated></entry></feed>`))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].URL != "https://go.dev/x" {
		t.Fatalf("items=%#v", items)
	}
}

func TestParseRSSUsesDetailedNamespacedContentAndDate(t *testing.T) {
	items, err := ParseFeed([]byte(`<rss xmlns:content="urn:content" xmlns:dc="urn:dc"><channel><item><title>Runtime update</title><link>https://example.com/runtime</link><description>Short teaser</description><content:encoded><![CDATA[<p>Detailed architecture and migration notes.</p>]]></content:encoded><dc:date>2026-08-05T10:00:00Z</dc:date></item></channel></rss>`))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Summary != "Detailed architecture and migration notes." || items[0].PublishedAt.IsZero() {
		t.Fatalf("items=%#v", items)
	}
}

func TestScoreRewardsRelevantTopic(t *testing.T) {
	relevant := Score("Terraform state security", "state locking and drift", []string{"Terraform", "state", "drift"}, 1, 2)
	irrelevant := Score("CSS colors", "design palettes", []string{"Terraform", "state", "drift"}, 1, 2)
	if relevant <= irrelevant {
		t.Fatalf("relevant=%f irrelevant=%f", relevant, irrelevant)
	}
}
