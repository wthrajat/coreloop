package radar

import "testing"

func TestClassifyNewsCategories(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		summary  string
		expected Category
	}{
		{"security", "Critical CVE-2026-12345", "A runtime vulnerability affects version 4.", CategorySecurity},
		{"research", "New inference research paper", "The technical report evaluates model scaling.", CategoryResearch},
		{"release", "Go 1.30 released", "The stable release changes the runtime.", CategoryRelease},
		{"funding", "Acme raises $50M Series B", "The funding round will expand its team.", CategoryFunding},
		{"partnership", "Acme partners with Example", "The partnership covers cloud infrastructure.", CategoryPartnership},
		{"pricing", "API pricing update", "Usage-based pricing changes next month.", CategoryPricing},
		{"industry", "DatabaseCo acquisition announced", "The acquisition is subject to approval.", CategoryIndustry},
		{"discussion", "Ask HN: How do you test distributed systems?", "Community thread.", CategoryDiscussion},
		{"product update", "EC2 instances now available in Paris", "This is a regional availability expansion.", CategoryProductUpdate},
		{"engineering", "Deep dive into our storage architecture", "A performance analysis of the implementation.", CategoryEngineering},
		{"default", "Cloudflare Wallets", "Programmable wallets for agents.", CategoryProductUpdate},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := Classify(test.title, test.summary); got != test.expected {
				t.Fatalf("Classify() = %q, want %q", got, test.expected)
			}
		})
	}
}

func TestCategoryLabelsAreNeutral(t *testing.T) {
	if got := CategoryProductUpdate.Label(); got != "Product update" {
		t.Fatalf("Label() = %q", got)
	}
	if got := CategoryFunding.Label(); got != "Funding" {
		t.Fatalf("Label() = %q", got)
	}
}
