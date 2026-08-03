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
func TestScoreRewardsRelevantTopic(t *testing.T) {
	relevant := Score("Terraform state security", "state locking and drift", []string{"Terraform", "state", "drift"}, 1, 2)
	irrelevant := Score("CSS colors", "design palettes", []string{"Terraform", "state", "drift"}, 1, 2)
	if relevant <= irrelevant {
		t.Fatalf("relevant=%f irrelevant=%f", relevant, irrelevant)
	}
}
