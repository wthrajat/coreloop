package jobs

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"coreloop/backend/internal/radar"
	"coreloop/backend/internal/store"
)

const (
	maximumSourceBodyBytes  = 8 << 20
	maximumSourceItems      = 50
	defaultAPIItemLimit     = 30
	sourceFetchConcurrency  = 6
	sourceChildFetchTimeout = 6 * time.Second
)

type sourceAdapterConfig struct {
	Adapter      string   `json:"adapter"`
	ItemLimit    int      `json:"item_limit"`
	PathPrefixes []string `json:"path_prefixes"`
	AllowedHosts []string `json:"allowed_hosts"`
}

type sourceFetchResult struct {
	Items          []radar.Item
	ETag           string
	LastModified   string
	NotModified    bool
	AttemptedItems int
	FailedItems    int
}

type sourceDocument struct {
	Body         []byte
	ETag         string
	LastModified string
	NotModified  bool
}

func (service *Service) fetchSource(ctx context.Context, source store.SourceRecord) (sourceFetchResult, error) {
	configuration := sourceAdapterConfig{Adapter: "feed", ItemLimit: defaultAPIItemLimit}
	if source.AdapterConfig != "" {
		if err := json.Unmarshal([]byte(source.AdapterConfig), &configuration); err != nil {
			return sourceFetchResult{}, fmt.Errorf("decode source adapter config: %w", err)
		}
	}
	if configuration.ItemLimit <= 0 || configuration.ItemLimit > maximumSourceItems {
		configuration.ItemLimit = defaultAPIItemLimit
	}

	var result sourceFetchResult
	var err error
	switch configuration.Adapter {
	case "feed", "":
		result, err = service.fetchFeed(ctx, source)
	case "github_releases":
		result, err = service.fetchGitHubReleases(ctx, source, configuration.ItemLimit)
	case "hacker_news":
		result, err = service.fetchHackerNews(ctx, source, configuration.ItemLimit)
	case "bluesky_author":
		result, err = service.fetchBlueskyAuthor(ctx, source, configuration.ItemLimit)
	case "sitemap":
		result, err = service.fetchSitemap(ctx, source, configuration)
	case "html_listing":
		result, err = service.fetchHTMLListing(ctx, source, configuration)
	default:
		return sourceFetchResult{}, fmt.Errorf("unsupported source adapter %q", configuration.Adapter)
	}
	if err == nil && !result.NotModified && result.AttemptedItems == 0 {
		result.AttemptedItems = len(result.Items)
	}
	return result, err
}

type blueskyAuthorFeed struct {
	Feed []struct {
		Post struct {
			URI    string `json:"uri"`
			Author struct {
				Handle      string `json:"handle"`
				DisplayName string `json:"displayName"`
			} `json:"author"`
			Record struct {
				Text      string `json:"text"`
				CreatedAt string `json:"createdAt"`
			} `json:"record"`
			Embed struct {
				External *struct {
					URI         string `json:"uri"`
					Title       string `json:"title"`
					Description string `json:"description"`
				} `json:"external"`
			} `json:"embed"`
			LikeCount   int `json:"likeCount"`
			RepostCount int `json:"repostCount"`
			ReplyCount  int `json:"replyCount"`
		} `json:"post"`
	} `json:"feed"`
}

func (service *Service) fetchBlueskyAuthor(ctx context.Context, source store.SourceRecord, limit int) (sourceFetchResult, error) {
	document, err := service.fetchDocument(ctx, source.URL, source.ETag, source.LastModified)
	if err != nil || document.NotModified {
		return sourceFetchResult{
			ETag: document.ETag, LastModified: document.LastModified,
			NotModified: document.NotModified,
		}, err
	}
	var feed blueskyAuthorFeed
	if err := json.Unmarshal(document.Body, &feed); err != nil {
		return sourceFetchResult{}, fmt.Errorf("parse Bluesky author feed: %w", err)
	}
	items := make([]radar.Item, 0, min(limit, len(feed.Feed)))
	for _, entry := range feed.Feed {
		postURL := blueskyPostURL(entry.Post.Author.Handle, entry.Post.URI)
		if postURL == "" {
			continue
		}
		title := ""
		summary := entry.Post.Record.Text
		canonical := postURL
		if entry.Post.Embed.External != nil {
			title = entry.Post.Embed.External.Title
			canonical = entry.Post.Embed.External.URI
			summary = strings.TrimSpace(entry.Post.Embed.External.Description + " " + summary)
		}
		if normalized, canonicalErr := radar.CanonicalURL(canonical); canonicalErr == nil {
			canonical = normalized
		} else {
			canonical = postURL
		}
		if title == "" {
			title = shortNewsTitle(summary)
		}
		if title == "" {
			continue
		}
		item := radar.Item{
			Title: title, URL: canonical, Summary: radar.NeutralText(summary),
			PublishedAt:               parseSourceTime(entry.Post.Record.CreatedAt),
			CommunityPoints:           entry.Post.LikeCount + entry.Post.RepostCount,
			CommunityComments:         entry.Post.ReplyCount,
			CommunitySignalsAvailable: true,
		}
		if canonical != postURL {
			displayName := firstNonEmpty(entry.Post.Author.DisplayName, entry.Post.Author.Handle, "Bluesky")
			item.DiscoveredVia = []radar.SourceReference{{Name: displayName + " on Bluesky", URL: postURL}}
		}
		items = append(items, item)
		if len(items) == limit {
			break
		}
	}
	return sourceFetchResult{Items: items, ETag: document.ETag, LastModified: document.LastModified}, nil
}

func blueskyPostURL(handle, uri string) string {
	const marker = "/app.bsky.feed.post/"
	position := strings.LastIndex(uri, marker)
	if strings.TrimSpace(handle) == "" || position < 0 {
		return ""
	}
	recordKey := strings.TrimSpace(uri[position+len(marker):])
	if recordKey == "" || strings.ContainsAny(recordKey, "/?#") {
		return ""
	}
	return "https://bsky.app/profile/" + url.PathEscape(handle) + "/post/" + url.PathEscape(recordKey)
}

func shortNewsTitle(value string) string {
	value = radar.NeutralText(value)
	if value == "" {
		return ""
	}
	if boundary := strings.IndexAny(value, "\n.!?"); boundary > 0 {
		value = value[:boundary]
	}
	runes := []rune(strings.TrimSpace(value))
	if len(runes) > 240 {
		runes = runes[:240]
	}
	return strings.TrimSpace(string(runes))
}

func (service *Service) fetchFeed(ctx context.Context, source store.SourceRecord) (sourceFetchResult, error) {
	document, err := service.fetchDocument(ctx, source.URL, source.ETag, source.LastModified)
	if err != nil || document.NotModified {
		return sourceFetchResult{
			ETag: document.ETag, LastModified: document.LastModified,
			NotModified: document.NotModified,
		}, err
	}
	items, err := radar.ParseFeed(document.Body)
	if err != nil {
		return sourceFetchResult{}, fmt.Errorf("parse source feed: %w", err)
	}
	if len(items) > maximumSourceItems {
		items = items[:maximumSourceItems]
	}
	for index := range items {
		items[index].URL = resolveSourceItemURL(source.URL, items[index].URL)
		if source.Role == "community_discovery" &&
			!sameSourceHostname(source.URL, items[index].URL) {
			items[index].DiscoveredVia = append(
				items[index].DiscoveredVia,
				radar.SourceReference{
					Name: source.Publisher + " feed",
					URL:  source.URL,
				},
			)
		}
	}
	return sourceFetchResult{Items: items, ETag: document.ETag, LastModified: document.LastModified}, nil
}

func sameSourceHostname(leftURL, rightURL string) bool {
	left, leftErr := url.Parse(leftURL)
	right, rightErr := url.Parse(rightURL)
	return leftErr == nil && rightErr == nil &&
		strings.EqualFold(left.Hostname(), right.Hostname())
}

func resolveSourceItemURL(sourceURL, itemURL string) string {
	base, baseErr := url.Parse(sourceURL)
	reference, referenceErr := url.Parse(strings.TrimSpace(itemURL))
	if baseErr != nil || referenceErr != nil {
		return itemURL
	}
	return base.ResolveReference(reference).String()
}

type githubRelease struct {
	Name        string    `json:"name"`
	TagName     string    `json:"tag_name"`
	HTMLURL     string    `json:"html_url"`
	Body        string    `json:"body"`
	PublishedAt time.Time `json:"published_at"`
	CreatedAt   time.Time `json:"created_at"`
	Draft       bool      `json:"draft"`
}

func (service *Service) fetchGitHubReleases(ctx context.Context, source store.SourceRecord, limit int) (sourceFetchResult, error) {
	document, err := service.fetchDocument(ctx, source.URL, source.ETag, source.LastModified)
	if err != nil || document.NotModified {
		return sourceFetchResult{
			ETag: document.ETag, LastModified: document.LastModified,
			NotModified: document.NotModified,
		}, err
	}
	var releases []githubRelease
	if err := json.Unmarshal(document.Body, &releases); err != nil {
		return sourceFetchResult{}, fmt.Errorf("parse GitHub releases: %w", err)
	}
	items := make([]radar.Item, 0, min(limit, len(releases)))
	for _, release := range releases {
		if release.Draft || release.HTMLURL == "" {
			continue
		}
		title := strings.TrimSpace(release.Name)
		if title == "" {
			title = strings.TrimSpace(release.TagName)
		}
		if title == "" {
			continue
		}
		publishedAt := release.PublishedAt
		if publishedAt.IsZero() {
			publishedAt = release.CreatedAt
		}
		items = append(items, radar.Item{
			Title: title, URL: release.HTMLURL,
			Summary: radar.NeutralText(release.Body), PublishedAt: publishedAt,
		})
		if len(items) == limit {
			break
		}
	}
	return sourceFetchResult{Items: items, ETag: document.ETag, LastModified: document.LastModified}, nil
}

type hackerNewsItem struct {
	ID          int    `json:"id"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Text        string `json:"text"`
	Time        int64  `json:"time"`
	Score       int    `json:"score"`
	Descendants int    `json:"descendants"`
	Deleted     bool   `json:"deleted"`
	Dead        bool   `json:"dead"`
}

func (service *Service) fetchHackerNews(ctx context.Context, source store.SourceRecord, limit int) (sourceFetchResult, error) {
	document, err := service.fetchDocument(ctx, source.URL, source.ETag, source.LastModified)
	if err != nil || document.NotModified {
		return sourceFetchResult{
			ETag: document.ETag, LastModified: document.LastModified,
			NotModified: document.NotModified,
		}, err
	}
	var ids []int
	if err := json.Unmarshal(document.Body, &ids); err != nil {
		return sourceFetchResult{}, fmt.Errorf("parse Hacker News story index: %w", err)
	}
	if len(ids) > limit {
		ids = ids[:limit]
	}
	items := make([]radar.Item, len(ids))
	valid := make([]bool, len(ids))
	semaphore := make(chan struct{}, sourceFetchConcurrency)
	var wait sync.WaitGroup
	for index, id := range ids {
		wait.Add(1)
		go func() {
			defer wait.Done()
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				return
			}
			itemURL := "https://hacker-news.firebaseio.com/v0/item/" + strconv.Itoa(id) + ".json"
			fetchContext, cancelFetch := context.WithTimeout(ctx, sourceChildFetchTimeout)
			itemDocument, fetchErr := service.fetchDocument(fetchContext, itemURL, "", "")
			cancelFetch()
			if fetchErr != nil {
				return
			}
			var value hackerNewsItem
			if json.Unmarshal(itemDocument.Body, &value) != nil || value.Deleted || value.Dead || value.Type != "story" || strings.TrimSpace(value.Title) == "" {
				return
			}
			discussionURL := "https://news.ycombinator.com/item?id=" + strconv.Itoa(value.ID)
			canonical := strings.TrimSpace(value.URL)
			if normalized, normalizeErr := radar.CanonicalURL(canonical); normalizeErr == nil {
				canonical = normalized
			} else {
				canonical = discussionURL
			}
			summary := radar.NeutralText(value.Text)
			if summary == "" && canonical != discussionURL {
				summary = "Hacker News discussion of the linked source."
			}
			items[index] = radar.Item{
				Title: value.Title, URL: canonical, Summary: summary,
				PublishedAt:               time.Unix(value.Time, 0).UTC(),
				CommunityPoints:           value.Score,
				CommunityComments:         value.Descendants,
				CommunitySignalsAvailable: true,
			}
			if canonical != discussionURL {
				items[index].DiscoveredVia = []radar.SourceReference{{Name: "Hacker News discussion", URL: discussionURL}}
			}
			valid[index] = true
		}()
	}
	wait.Wait()
	if err := ctx.Err(); err != nil {
		return sourceFetchResult{}, err
	}
	output := make([]radar.Item, 0, len(items))
	for index := range items {
		if valid[index] {
			output = append(output, items[index])
		}
	}
	if len(ids) > 0 && len(output) == 0 {
		return sourceFetchResult{}, errors.New("Hacker News item requests returned no usable stories")
	}
	return sourceFetchResult{
		Items: output, ETag: document.ETag, LastModified: document.LastModified,
		AttemptedItems: len(ids), FailedItems: len(ids) - len(output),
	}, nil
}

type sitemapDocument struct {
	URLs []struct {
		Location string `xml:"loc"`
		LastMod  string `xml:"lastmod"`
	} `xml:"url"`
}

type sitemapPage struct {
	URL     string
	LastMod time.Time
}

func (service *Service) fetchSitemap(ctx context.Context, source store.SourceRecord, configuration sourceAdapterConfig) (sourceFetchResult, error) {
	document, err := service.fetchDocument(ctx, source.URL, source.ETag, source.LastModified)
	if err != nil || document.NotModified {
		return sourceFetchResult{
			ETag: document.ETag, LastModified: document.LastModified,
			NotModified: document.NotModified,
		}, err
	}
	var sitemap sitemapDocument
	if err := xml.Unmarshal(document.Body, &sitemap); err != nil {
		return sourceFetchResult{}, fmt.Errorf("parse source sitemap: %w", err)
	}
	sourceURL, _ := url.Parse(source.URL)
	pages := make([]sitemapPage, 0, len(sitemap.URLs))
	for _, entry := range sitemap.URLs {
		pageURL, parseErr := url.Parse(strings.TrimSpace(entry.Location))
		if parseErr != nil || pageURL.Scheme != "https" || !strings.EqualFold(pageURL.Hostname(), sourceURL.Hostname()) {
			continue
		}
		if !hasPathPrefix(pageURL.Path, configuration.PathPrefixes) {
			continue
		}
		pages = append(pages, sitemapPage{URL: pageURL.String(), LastMod: parseSourceTime(entry.LastMod)})
	}
	sort.SliceStable(pages, func(left, right int) bool {
		return pages[left].LastMod.After(pages[right].LastMod)
	})
	if len(pages) > configuration.ItemLimit {
		pages = pages[:configuration.ItemLimit]
	}
	items, failedItems, err := service.fetchMetadataPages(ctx, pages, false)
	if err != nil {
		return sourceFetchResult{}, err
	}
	return sourceFetchResult{
		Items: items, ETag: document.ETag, LastModified: document.LastModified,
		AttemptedItems: len(pages), FailedItems: failedItems,
	}, nil
}

func (service *Service) fetchHTMLListing(ctx context.Context, source store.SourceRecord, configuration sourceAdapterConfig) (sourceFetchResult, error) {
	document, err := service.fetchDocument(ctx, source.URL, source.ETag, source.LastModified)
	if err != nil || document.NotModified {
		return sourceFetchResult{
			ETag: document.ETag, LastModified: document.LastModified,
			NotModified: document.NotModified,
		}, err
	}
	pages := listingPages(source.URL, document.Body, configuration)
	if len(pages) == 0 {
		return sourceFetchResult{}, errors.New("HTML listing contains no eligible page links")
	}
	if len(pages) > configuration.ItemLimit {
		pages = pages[:configuration.ItemLimit]
	}
	items, failedItems, err := service.fetchMetadataPages(ctx, pages, true)
	if err != nil {
		return sourceFetchResult{}, err
	}
	return sourceFetchResult{
		Items: items, ETag: document.ETag, LastModified: document.LastModified,
		AttemptedItems: len(pages), FailedItems: failedItems,
	}, nil
}

func listingPages(rawSourceURL string, body []byte, configuration sourceAdapterConfig) []sitemapPage {
	sourceURL, err := url.Parse(rawSourceURL)
	if err != nil {
		return nil
	}
	allowedHosts := map[string]struct{}{strings.ToLower(sourceURL.Hostname()): {}}
	for _, host := range configuration.AllowedHosts {
		host = strings.ToLower(strings.TrimSpace(host))
		if host != "" {
			allowedHosts[host] = struct{}{}
		}
	}
	seen := make(map[string]struct{})
	pages := make([]sitemapPage, 0, configuration.ItemLimit)
	for _, tag := range anchorTagPattern.FindAllString(string(body), -1) {
		attributes := parseHTMLAttributes(tag)
		reference, parseErr := url.Parse(strings.TrimSpace(attributes["href"]))
		if parseErr != nil || reference.String() == "" {
			continue
		}
		pageURL := sourceURL.ResolveReference(reference)
		if pageURL.Scheme != "https" {
			continue
		}
		if _, allowed := allowedHosts[strings.ToLower(pageURL.Hostname())]; !allowed {
			continue
		}
		if !hasPathPrefix(pageURL.Path, configuration.PathPrefixes) {
			continue
		}
		pageURL.Fragment = ""
		resolved := pageURL.String()
		if sameWebPage(rawSourceURL, resolved) {
			continue
		}
		if _, duplicate := seen[resolved]; duplicate {
			continue
		}
		seen[resolved] = struct{}{}
		pages = append(pages, sitemapPage{URL: resolved})
		if len(pages) == maximumSourceItems {
			break
		}
	}
	return pages
}

func sameWebPage(leftRawURL, rightRawURL string) bool {
	left, leftErr := url.Parse(leftRawURL)
	right, rightErr := url.Parse(rightRawURL)
	if leftErr != nil || rightErr != nil {
		return false
	}
	return strings.EqualFold(left.Hostname(), right.Hostname()) &&
		strings.TrimRight(left.Path, "/") == strings.TrimRight(right.Path, "/")
}

func (service *Service) fetchMetadataPages(ctx context.Context, pages []sitemapPage, requirePublishedTime bool) ([]radar.Item, int, error) {
	items := make([]radar.Item, len(pages))
	valid := make([]bool, len(pages))
	semaphore := make(chan struct{}, sourceFetchConcurrency)
	var wait sync.WaitGroup
	for index, page := range pages {
		wait.Add(1)
		go func() {
			defer wait.Done()
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				return
			}
			fetchContext, cancelFetch := context.WithTimeout(ctx, sourceChildFetchTimeout)
			pageDocument, fetchErr := service.fetchDocument(fetchContext, page.URL, "", "")
			cancelFetch()
			if fetchErr != nil {
				return
			}
			title, summary, publishedAt := parsePageMetadata(pageDocument.Body)
			if title == "" {
				title = titleFromPath(page.URL)
			}
			if publishedAt.IsZero() {
				publishedAt = page.LastMod
			}
			if title == "" || (requirePublishedTime && publishedAt.IsZero()) {
				return
			}
			items[index] = radar.Item{Title: title, URL: page.URL, Summary: summary, PublishedAt: publishedAt}
			valid[index] = true
		}()
	}
	wait.Wait()
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	output := make([]radar.Item, 0, len(items))
	for index := range items {
		if valid[index] {
			output = append(output, items[index])
		}
	}
	if len(pages) > 0 && len(output) == 0 {
		return nil, len(pages), errors.New("source pages returned no usable metadata")
	}
	return output, len(pages) - len(output), nil
}

type sourceHTTPError struct {
	StatusCode int
}

func (failure sourceHTTPError) Error() string {
	return fmt.Sprintf("source returned HTTP %d", failure.StatusCode)
}

func (service *Service) fetchDocument(ctx context.Context, rawURL, etag, lastModified string) (sourceDocument, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return sourceDocument{}, err
	}
	if err := validateSourceURL(request.URL); err != nil {
		return sourceDocument{}, err
	}
	request.Header.Set("User-Agent", "Coreloop/1.0 (+"+service.appOrigin+")")
	request.Header.Set("Accept", "application/atom+xml, application/rss+xml, application/xml, application/json, text/html;q=0.9, */*;q=0.5")
	if etag != "" {
		request.Header.Set("If-None-Match", etag)
	}
	if lastModified != "" {
		request.Header.Set("If-Modified-Since", lastModified)
	}
	response, err := service.http.Do(request)
	if err != nil {
		return sourceDocument{}, err
	}
	defer response.Body.Close()
	result := sourceDocument{
		ETag: response.Header.Get("ETag"), LastModified: response.Header.Get("Last-Modified"),
	}
	if response.StatusCode == http.StatusNotModified {
		result.NotModified = true
		if result.ETag == "" {
			result.ETag = etag
		}
		if result.LastModified == "" {
			result.LastModified = lastModified
		}
		return result, nil
	}
	if response.StatusCode != http.StatusOK {
		return sourceDocument{}, sourceHTTPError{StatusCode: response.StatusCode}
	}
	result.Body, err = io.ReadAll(io.LimitReader(response.Body, maximumSourceBodyBytes+1))
	if err != nil {
		return sourceDocument{}, err
	}
	if len(result.Body) > maximumSourceBodyBytes {
		return sourceDocument{}, errors.New("source response exceeds 8 MiB")
	}
	return result, nil
}

func sourcePollDiagnostic(err error) (string, string) {
	if errors.Is(err, context.DeadlineExceeded) {
		return "source_timeout", "The source did not respond before the fetch deadline."
	}
	var networkError net.Error
	if errors.As(err, &networkError) {
		return "source_network_error", "The source could not be reached over the network."
	}
	var httpError sourceHTTPError
	if errors.As(err, &httpError) {
		return "source_http_error", fmt.Sprintf(
			"The source returned HTTP %d.",
			httpError.StatusCode,
		)
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "unsupported source adapter") ||
		strings.Contains(message, "decode source adapter config"):
		return "source_configuration_invalid", "The source adapter configuration is invalid."
	case strings.Contains(message, "parse"):
		return "source_parse_failed", "The source response could not be parsed in its configured format."
	case strings.Contains(message, "no usable") || strings.Contains(message, "no eligible"):
		return "source_items_unavailable", "The source index loaded, but its item requests returned no usable stories."
	case strings.Contains(message, "exceeds 8 mib"):
		return "source_response_too_large", "The source response exceeded the safe size limit."
	default:
		return "source_poll_failed", "The source could not be fetched or processed."
	}
}

func hasPathPrefix(path string, prefixes []string) bool {
	if len(prefixes) == 0 {
		return true
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

var (
	metaTagPattern   = regexp.MustCompile(`(?is)<meta\s+[^>]*>`)
	anchorTagPattern = regexp.MustCompile(`(?is)<a\s+[^>]*>`)
	metaAttrPattern  = regexp.MustCompile(`(?i)([a-z_:.-]+)\s*=\s*["']([^"']*)["']`)
	titleTagPattern  = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	jsonLDPattern    = regexp.MustCompile(`(?is)<script[^>]*type\s*=\s*["']application/ld\+json["'][^>]*>(.*?)</script>`)
)

func parseHTMLAttributes(tag string) map[string]string {
	attributes := map[string]string{}
	for _, match := range metaAttrPattern.FindAllStringSubmatch(tag, -1) {
		attributes[strings.ToLower(match[1])] = html.UnescapeString(strings.TrimSpace(match[2]))
	}
	return attributes
}

func parsePageMetadata(body []byte) (string, string, time.Time) {
	metadata := map[string]string{}
	for _, tag := range metaTagPattern.FindAllString(string(body), -1) {
		attributes := parseHTMLAttributes(tag)
		key := strings.ToLower(attributes["property"])
		if key == "" {
			key = strings.ToLower(attributes["name"])
		}
		if key != "" && attributes["content"] != "" {
			metadata[key] = attributes["content"]
		}
	}
	structuredTitle, structuredSummary, structuredDate := parseJSONLDMetadata(body)
	title := firstNonEmpty(metadata["og:title"], metadata["twitter:title"], structuredTitle)
	if title == "" {
		if match := titleTagPattern.FindStringSubmatch(string(body)); len(match) == 2 {
			title = radar.NeutralText(match[1])
		}
	}
	summary := firstNonEmpty(metadata["og:description"], metadata["twitter:description"], metadata["description"], structuredSummary)
	published := parseSourceTime(firstNonEmpty(metadata["article:published_time"], metadata["date"], metadata["datepublished"], structuredDate))
	return radar.NeutralText(title), radar.NeutralText(summary), published
}

func parseJSONLDMetadata(body []byte) (string, string, string) {
	for _, match := range jsonLDPattern.FindAllSubmatch(body, -1) {
		if len(match) != 2 {
			continue
		}
		var value any
		if json.Unmarshal(match[1], &value) != nil {
			continue
		}
		title := findJSONLDString(value, "headline", 0)
		summary := findJSONLDString(value, "description", 0)
		published := findJSONLDString(value, "datePublished", 0)
		if title != "" || summary != "" || published != "" {
			return title, summary, published
		}
	}
	return "", "", ""
}

func findJSONLDString(value any, key string, depth int) string {
	if depth > 32 {
		return ""
	}
	switch typed := value.(type) {
	case map[string]any:
		if direct, ok := typed[key].(string); ok {
			return strings.TrimSpace(direct)
		}
		for _, nested := range typed {
			if found := findJSONLDString(nested, key, depth+1); found != "" {
				return found
			}
		}
	case []any:
		for _, nested := range typed {
			if found := findJSONLDString(nested, key, depth+1); found != "" {
				return found
			}
		}
	}
	return ""
}

func parseSourceTime(value string) time.Time {
	value = strings.TrimSpace(value)
	for _, layout := range []string{time.RFC3339, "2006-01-02", time.RFC1123Z, time.RFC1123} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}

func titleFromPath(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) == 0 {
		return ""
	}
	title := strings.NewReplacer("-", " ", "_", " ").Replace(segments[len(segments)-1])
	return strings.TrimSpace(title)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
