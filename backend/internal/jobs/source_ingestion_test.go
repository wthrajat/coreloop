package jobs

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"coreloop/backend/internal/store"
)

func TestFetchGitHubReleasesUsesOfficialReleaseMetadata(t *testing.T) {
	service := sourceTestService(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/repos/example/project/releases" {
			t.Fatalf("request path = %q", request.URL.Path)
		}
		return sourceResponse(http.StatusOK, `[{"name":"Project 2.0","tag_name":"v2.0.0","html_url":"https://github.com/example/project/releases/tag/v2.0.0","body":"We are excited to announce a world-class release. Adds a stable API. Register now.","published_at":"2026-08-05T06:00:00Z","draft":false},{"name":"Draft","html_url":"https://github.com/example/project/releases/tag/draft","draft":true}]`), nil
	})
	result, err := service.fetchSource(context.Background(), store.SourceRecord{
		URL:           "https://api.github.com/repos/example/project/releases",
		AdapterConfig: `{"adapter":"github_releases"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("items = %#v", result.Items)
	}
	item := result.Items[0]
	if item.Title != "Project 2.0" || !strings.Contains(item.Summary, "Adds a stable API") {
		t.Fatalf("release item = %#v", item)
	}
	if strings.Contains(strings.ToLower(item.Summary), "excited") || strings.Contains(strings.ToLower(item.Summary), "register") {
		t.Fatalf("release summary was not neutralized: %q", item.Summary)
	}
}

func TestFetchHackerNewsKeepsOriginalAndDiscussionSources(t *testing.T) {
	service := sourceTestService(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/v0/beststories.json":
			return sourceResponse(http.StatusOK, `[42]`), nil
		case "/v0/item/42.json":
			return sourceResponse(http.StatusOK, `{"id":42,"type":"story","title":"A useful database paper","url":"https://example.com/paper?utm_source=hn","time":1785913200,"score":321,"descendants":87}`), nil
		default:
			t.Fatalf("unexpected path %q", request.URL.Path)
			return nil, nil
		}
	})
	result, err := service.fetchSource(context.Background(), store.SourceRecord{
		URL:           "https://hacker-news.firebaseio.com/v0/beststories.json",
		AdapterConfig: `{"adapter":"hacker_news","item_limit":5}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("items = %#v", result.Items)
	}
	item := result.Items[0]
	if item.URL != "https://example.com/paper" || item.CommunityPoints != 321 ||
		item.CommunityComments != 87 || !item.CommunitySignalsAvailable {
		t.Fatalf("HN item = %#v", item)
	}
	if len(item.DiscoveredVia) != 1 || item.DiscoveredVia[0].URL != "https://news.ycombinator.com/item?id=42" {
		t.Fatalf("HN discovery = %#v", item.DiscoveredVia)
	}
}

func TestFetchBlueskyAuthorUsesEmbeddedOriginalSource(t *testing.T) {
	service := sourceTestService(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/xrpc/app.bsky.feed.getAuthorFeed" {
			t.Fatalf("request path = %q", request.URL.Path)
		}
		return sourceResponse(http.StatusOK, `{"feed":[{"post":{"uri":"at://did:plc:test/app.bsky.feed.post/abc123","author":{"handle":"example.com","displayName":"Example Research"},"record":{"text":"Technical details and discussion.","createdAt":"2026-08-05T08:00:00Z"},"embed":{"external":{"uri":"https://example.com/research/new-model?utm_source=bluesky","title":"A new model architecture","description":"The paper describes a sparse runtime."}},"likeCount":90,"repostCount":20,"replyCount":15}}]}`), nil
	})
	result, err := service.fetchSource(context.Background(), store.SourceRecord{
		URL:           "https://public.api.bsky.app/xrpc/app.bsky.feed.getAuthorFeed?actor=example.com&limit=30",
		AdapterConfig: `{"adapter":"bluesky_author","item_limit":10}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("items = %#v", result.Items)
	}
	item := result.Items[0]
	if item.URL != "https://example.com/research/new-model" || item.CommunityPoints != 110 ||
		item.CommunityComments != 15 || !item.CommunitySignalsAvailable {
		t.Fatalf("Bluesky item = %#v", item)
	}
	if len(item.DiscoveredVia) != 1 || item.DiscoveredVia[0].URL != "https://bsky.app/profile/example.com/post/abc123" {
		t.Fatalf("Bluesky discovery = %#v", item.DiscoveredVia)
	}
}

func TestFetchSitemapFiltersPathsAndReadsNeutralPageMetadata(t *testing.T) {
	service := sourceTestService(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/sitemap.xml":
			return sourceResponse(http.StatusOK, `<?xml version="1.0"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><url><loc>https://example.com/news/new-runtime</loc><lastmod>2026-08-05</lastmod></url><url><loc>https://example.com/company/careers</loc><lastmod>2026-08-06</lastmod></url></urlset>`), nil
		case "/news/new-runtime":
			return sourceResponse(http.StatusOK, `<html><head><meta property="og:title" content="New runtime released"><meta name="description" content="We are thrilled to announce a cutting-edge runtime. It reduces cold starts by 40%. Read more."><meta property="article:published_time" content="2026-08-05T07:00:00Z"></head></html>`), nil
		default:
			t.Fatalf("unexpected path %q", request.URL.Path)
			return nil, nil
		}
	})
	result, err := service.fetchSource(context.Background(), store.SourceRecord{
		URL:           "https://example.com/sitemap.xml",
		AdapterConfig: `{"adapter":"sitemap","item_limit":5,"path_prefixes":["/news/"]}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("items = %#v", result.Items)
	}
	if result.Items[0].Title != "New runtime released" || result.Items[0].Summary != "a runtime. It reduces cold starts by 40%." {
		t.Fatalf("sitemap item = %#v", result.Items[0])
	}
}

func TestFetchFeedPreservesConditionalRequestState(t *testing.T) {
	service := sourceTestService(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("If-None-Match") != `"feed-v1"` {
			t.Fatalf("If-None-Match = %q", request.Header.Get("If-None-Match"))
		}
		response := sourceResponse(http.StatusNotModified, "")
		response.Header.Set("ETag", `"feed-v1"`)
		return response, nil
	})
	result, err := service.fetchSource(context.Background(), store.SourceRecord{
		URL: "https://example.com/feed.xml", ETag: `"feed-v1"`,
		AdapterConfig: `{"adapter":"feed"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.NotModified || result.ETag != `"feed-v1"` {
		t.Fatalf("conditional result = %#v", result)
	}
}

func TestFetchFeedResolvesRelativeArticleLinks(t *testing.T) {
	service := sourceTestService(func(*http.Request) (*http.Response, error) {
		return sourceResponse(http.StatusOK, `<rss><channel><item><title>Update</title><link>/news/update</link><description>Details</description></item></channel></rss>`), nil
	})
	result, err := service.fetchSource(context.Background(), store.SourceRecord{
		URL: "https://example.com/feed.xml", AdapterConfig: `{"adapter":"feed"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 || result.Items[0].URL != "https://example.com/news/update" {
		t.Fatalf("items = %#v", result.Items)
	}
}

func sourceTestService(roundTrip jobRoundTripFunc) *Service {
	return &Service{
		http:      &http.Client{Transport: roundTrip, Timeout: time.Second},
		appOrigin: "https://coreloop.example",
	}
}

func sourceResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}
