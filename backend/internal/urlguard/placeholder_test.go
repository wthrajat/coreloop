package urlguard

import "testing"

func TestIsPlaceholderHost(t *testing.T) {
	for _, host := range []string{
		"example.com",
		"news.example.com",
		"docs.example",
		"placeholder.invalid",
		"service.test",
		"localhost",
	} {
		if !IsPlaceholderHost(host) {
			t.Fatalf("expected %q to be rejected", host)
		}
	}
	for _, host := range []string{"openai.com", "go.dev", "example.co"} {
		if IsPlaceholderHost(host) {
			t.Fatalf("expected %q to be accepted", host)
		}
	}
}
