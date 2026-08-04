package radar

import (
	"strings"
	"testing"
)

func TestCanonicalURLNormalizesHTTPSAndRemovesTracking(t *testing.T) {
	got, err := CanonicalURL("http://Example.COM:80/posts/runtime/?utm_source=hackernews&id=7&fbclid=ignored#comments")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://example.com/posts/runtime/?id=7"
	if got != want {
		t.Fatalf("CanonicalURL() = %q, want %q", got, want)
	}
}

func TestCanonicalURLPreservesContentParameters(t *testing.T) {
	got, err := CanonicalURL("example.com/story?source=weekly&version=2")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://example.com/story?source=weekly&version=2"
	if got != want {
		t.Fatalf("CanonicalURL() = %q, want %q", got, want)
	}
}

func TestHackerNewsOutboundURLClustersWithOriginal(t *testing.T) {
	fromHackerNews := "https://openai.com/research/example?utm_source=hackernews&utm_medium=referral"
	original := "https://openai.com/research/example"
	if ClusterKey(fromHackerNews, "HN title") != ClusterKey(original, "Original title") {
		t.Fatal("outbound Hacker News link did not cluster with the original URL")
	}
}

func TestCanonicalURLRejectsMalformedURLs(t *testing.T) {
	for _, value := range []string{"", "https://", "ftp://example.com/file", "https://user:secret@example.com/post", "https://bad host/post"} {
		t.Run(strings.ReplaceAll(value, "/", "_"), func(t *testing.T) {
			if _, err := CanonicalURL(value); err == nil {
				t.Fatalf("CanonicalURL(%q) unexpectedly succeeded", value)
			}
		})
	}
}

func TestClusterKeyFallsBackToNormalizedTitle(t *testing.T) {
	first := ClusterKey("not a valid URL %", "Go 1.30: Runtime Changes!")
	second := ClusterKey("", "  GO 1.30 — runtime changes  ")
	if first == "" || first != second {
		t.Fatalf("fallback keys differ: %q != %q", first, second)
	}
	if !strings.HasPrefix(first, "title:") {
		t.Fatalf("fallback key = %q, want title prefix", first)
	}
}

func TestClusterKeyDoesNotMergeDifferentValidURLs(t *testing.T) {
	first := ClusterKey("https://one.example/post", "Same announcement")
	second := ClusterKey("https://two.example/post", "Same announcement")
	if first == second {
		t.Fatal("valid source URLs should take precedence over duplicate titles")
	}
}
