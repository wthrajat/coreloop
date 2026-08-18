package jobs

import (
	"context"
	"io"
	"net/http"
	"net/url"
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

func TestSourceURLRejectsNonPublicLiteralAddresses(t *testing.T) {
	for _, rawURL := range []string{
		"https://127.0.0.1/feed", "https://10.0.0.2/feed",
		"https://0.0.0.0/feed", "https://100.64.0.1/feed",
		"https://192.0.2.1/feed", "https://224.0.0.1/feed", "https://[::1]/feed",
	} {
		parsed, err := url.Parse(rawURL)
		if err != nil {
			t.Fatal(err)
		}
		if err := validateSourceURL(parsed); err == nil {
			t.Fatalf("non-public source was accepted: %s", rawURL)
		}
	}
}

func TestFetchHackerNewsKeepsOriginalAndDiscussionSources(t *testing.T) {
	service := sourceTestService(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/v0/beststories.json":
			return sourceResponse(http.StatusOK, `[42]`), nil
		case "/v0/item/42.json":
			return sourceResponse(http.StatusOK, `{"id":42,"type":"story","title":"A useful database paper","url":"https://example.co/paper?utm_source=hn","time":1785913200,"score":321,"descendants":87}`), nil
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
	if item.URL != "https://example.co/paper" || item.CommunityPoints != 321 ||
		item.CommunityComments != 87 || !item.CommunitySignalsAvailable {
		t.Fatalf("HN item = %#v", item)
	}
	if len(item.DiscoveredVia) != 1 || item.DiscoveredVia[0].URL != "https://news.ycombinator.com/item?id=42" {
		t.Fatalf("HN discovery = %#v", item.DiscoveredVia)
	}
}

func TestFetchHackerNewsReportsPartialChildFailures(t *testing.T) {
	service := sourceTestService(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/v0/beststories.json":
			return sourceResponse(http.StatusOK, `[42,43]`), nil
		case "/v0/item/42.json":
			return sourceResponse(http.StatusOK, `{"id":42,"type":"story","title":"Database reliability notes","url":"https://example.co/reliability","time":1785913200,"score":100,"descendants":20}`), nil
		case "/v0/item/43.json":
			return sourceResponse(http.StatusTooManyRequests, ``), nil
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
	if len(result.Items) != 1 || result.AttemptedItems != 2 || result.FailedItems != 1 {
		t.Fatalf("partial HN result = %#v", result)
	}
}

func TestSourcePollDiagnosticDoesNotExposeResponseDetails(t *testing.T) {
	code, summary := sourcePollDiagnostic(sourceHTTPError{StatusCode: http.StatusTooManyRequests})
	if code != "source_http_error" || summary != "The source returned HTTP 429." {
		t.Fatalf("diagnostic = %q %q", code, summary)
	}
}

func TestFetchBlueskyAuthorUsesEmbeddedOriginalSource(t *testing.T) {
	service := sourceTestService(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/xrpc/app.bsky.feed.getAuthorFeed" {
			t.Fatalf("request path = %q", request.URL.Path)
		}
		return sourceResponse(http.StatusOK, `{"feed":[{"post":{"uri":"at://did:plc:test/app.bsky.feed.post/abc123","author":{"handle":"example.co","displayName":"Example Research"},"record":{"text":"Technical details and discussion.","createdAt":"2026-08-05T08:00:00Z"},"embed":{"external":{"uri":"https://example.co/research/new-model?utm_source=bluesky","title":"A new model architecture","description":"The paper describes a sparse runtime."}},"likeCount":90,"repostCount":20,"replyCount":15}}]}`), nil
	})
	result, err := service.fetchSource(context.Background(), store.SourceRecord{
		URL:           "https://public.api.bsky.app/xrpc/app.bsky.feed.getAuthorFeed?actor=example.co&limit=30",
		AdapterConfig: `{"adapter":"bluesky_author","item_limit":10}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("items = %#v", result.Items)
	}
	item := result.Items[0]
	if item.URL != "https://example.co/research/new-model" || item.CommunityPoints != 110 ||
		item.CommunityComments != 15 || !item.CommunitySignalsAvailable {
		t.Fatalf("Bluesky item = %#v", item)
	}
	if len(item.DiscoveredVia) != 1 || item.DiscoveredVia[0].URL != "https://bsky.app/profile/example.co/post/abc123" {
		t.Fatalf("Bluesky discovery = %#v", item.DiscoveredVia)
	}
}

func TestFetchSitemapFiltersPathsAndReadsNeutralPageMetadata(t *testing.T) {
	service := sourceTestService(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/sitemap.xml":
			return sourceResponse(http.StatusOK, `<?xml version="1.0"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><url><loc>https://example.co/news/new-runtime</loc><lastmod>2026-08-05</lastmod></url><url><loc>https://example.co/company/careers</loc><lastmod>2026-08-06</lastmod></url></urlset>`), nil
		case "/news/new-runtime":
			return sourceResponse(http.StatusOK, `<html><head><meta property="og:title" content="New runtime released"><meta name="description" content="We are thrilled to announce a cutting-edge runtime. It reduces cold starts by 40%. Read more."><meta property="article:published_time" content="2026-08-05T07:00:00Z"></head></html>`), nil
		default:
			t.Fatalf("unexpected path %q", request.URL.Path)
			return nil, nil
		}
	})
	result, err := service.fetchSource(context.Background(), store.SourceRecord{
		URL:           "https://example.co/sitemap.xml",
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

func TestFetchHTMLListingFollowsOnlyAllowlistedArticleLinks(t *testing.T) {
	service := sourceTestService(func(request *http.Request) (*http.Response, error) {
		switch request.URL.String() {
		case "https://example.co/engineering/":
			return sourceResponse(http.StatusOK, `<html><body>
				<a href="/engineering/">Listing</a>
				<a href="/engineering/runtime?utm_source=navigation#details">Runtime</a>
				<a href="/engineering/runtime?utm_source=navigation#comments">Duplicate</a>
				<a href="https://research.example.co/engineering/paper">Paper</a>
				<a href="https://untrusted.example/engineering/trap">Untrusted</a>
				<a href="/careers">Careers</a>
			</body></html>`), nil
		case "https://example.co/engineering/runtime?utm_source=navigation":
			return sourceResponse(http.StatusOK, `<html><head><meta property="og:title" content="A faster runtime"><meta name="description" content="We are excited to announce a new runtime. It cuts startup latency by 40%."><meta property="article:published_time" content="2026-08-05T07:00:00Z"></head></html>`), nil
		case "https://research.example.co/engineering/paper":
			return sourceResponse(http.StatusOK, `<html><head><title>Distributed systems paper</title><meta name="description" content="A new consistency protocol."><meta name="date" content="2026-08-04"></head></html>`), nil
		default:
			t.Fatalf("unexpected URL %q", request.URL.String())
			return nil, nil
		}
	})
	result, err := service.fetchSource(context.Background(), store.SourceRecord{
		URL:           "https://example.co/engineering/",
		AdapterConfig: `{"adapter":"html_listing","item_limit":5,"path_prefixes":["/engineering/"],"allowed_hosts":["research.example.co"]}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 2 || result.AttemptedItems != 2 || result.FailedItems != 0 {
		t.Fatalf("HTML listing result = %#v", result)
	}
	if result.Items[0].Title != "A faster runtime" ||
		result.Items[0].Summary != "a new runtime. It cuts startup latency by 40%." {
		t.Fatalf("first listing item = %#v", result.Items[0])
	}
	if result.Items[1].URL != "https://research.example.co/engineering/paper" {
		t.Fatalf("allowlisted cross-host item = %#v", result.Items[1])
	}
}

func TestFetchHTMLListingFailsWhenNoEligibleLinksExist(t *testing.T) {
	service := sourceTestService(func(*http.Request) (*http.Response, error) {
		return sourceResponse(http.StatusOK, `<a href="http://127.0.0.1/private">Private</a><a href="https://other.example/news">Other host</a>`), nil
	})
	_, err := service.fetchSource(context.Background(), store.SourceRecord{
		URL:           "https://example.co/engineering/",
		AdapterConfig: `{"adapter":"html_listing","path_prefixes":["/engineering/"]}`,
	})
	if err == nil || !strings.Contains(err.Error(), "no eligible page links") {
		t.Fatalf("error = %v", err)
	}
}

func TestParsePageMetadataUsesJSONLDArticleFields(t *testing.T) {
	title, summary, publishedAt := parsePageMetadata([]byte(`<html><head>
		<script type="application/ld+json">{"@type":"TechArticle","headline":"Storage internals","description":"How the engine persists writes.","datePublished":"2026-08-03T09:30:00Z"}</script>
	</head></html>`))
	if title != "Storage internals" || summary != "How the engine persists writes." {
		t.Fatalf("JSON-LD metadata = %q %q", title, summary)
	}
	if publishedAt.Format(time.RFC3339) != "2026-08-03T09:30:00Z" {
		t.Fatalf("published at = %s", publishedAt)
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
		URL: "https://example.co/feed.xml", ETag: `"feed-v1"`,
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
		URL: "https://example.co/feed.xml", AdapterConfig: `{"adapter":"feed"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 || result.Items[0].URL != "https://example.co/news/update" {
		t.Fatalf("items = %#v", result.Items)
	}
}

func TestCommunityFeedPreservesDiscoveryProvenanceForOutboundLinks(t *testing.T) {
	service := sourceTestService(func(*http.Request) (*http.Response, error) {
		return sourceResponse(http.StatusOK, `<rss><channel><item><title>Database analysis</title><link>https://example.co/database</link><description>Technical details</description></item></channel></rss>`), nil
	})
	result, err := service.fetchSource(context.Background(), store.SourceRecord{
		Publisher:     "Stacker News · Tech",
		URL:           "https://stacker.news/~tech/rss",
		Role:          "community_discovery",
		AdapterConfig: `{"adapter":"feed"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 || len(result.Items[0].DiscoveredVia) != 1 {
		t.Fatalf("community discovery provenance = %#v", result.Items)
	}
	if result.Items[0].DiscoveredVia[0].URL != "https://stacker.news/~tech/rss" {
		t.Fatalf("discovery = %#v", result.Items[0].DiscoveredVia)
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
