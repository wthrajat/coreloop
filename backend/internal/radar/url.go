package radar

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"net/url"
	"regexp"
	"strings"
)

var normalizedTitleWords = regexp.MustCompile(`[a-z0-9]+`)

var trackingQueryParameters = map[string]bool{
	"_hsenc":      true,
	"_hsmi":       true,
	"dclid":       true,
	"fbclid":      true,
	"gclid":       true,
	"igshid":      true,
	"mc_cid":      true,
	"mc_eid":      true,
	"mkt_tok":     true,
	"msclkid":     true,
	"oly_anon_id": true,
	"oly_enc_id":  true,
	"rb_clickid":  true,
	"s_cid":       true,
	"twclid":      true,
	"vero_conv":   true,
	"vero_id":     true,
	"wickedid":    true,
}

// CanonicalURL returns a stable HTTPS representation of a news URL. It keeps
// content-identifying query parameters and removes only known tracking data.
func CanonicalURL(rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", errors.New("news URL is empty")
	}
	if strings.HasPrefix(rawURL, "//") {
		rawURL = "https:" + rawURL
	} else if !strings.Contains(rawURL, "://") {
		rawURL = "https://" + rawURL
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", errors.New("news URL is malformed")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("news URL must use HTTP or HTTPS")
	}
	if parsed.User != nil {
		return "", errors.New("news URL must not contain credentials")
	}

	hostname := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if hostname == "" || strings.ContainsAny(hostname, " \t\r\n") {
		return "", errors.New("news URL must contain a valid host")
	}
	port := parsed.Port()
	if port == "80" || port == "443" {
		port = ""
	}
	if port != "" {
		parsed.Host = net.JoinHostPort(hostname, port)
	} else if strings.Contains(hostname, ":") {
		parsed.Host = "[" + hostname + "]"
	} else {
		parsed.Host = hostname
	}

	parsed.Scheme = "https"
	parsed.Fragment = ""
	parsed.RawFragment = ""
	parsed.RawQuery = canonicalQuery(parsed.Query()).Encode()
	if parsed.Path == "/" {
		parsed.Path = ""
		parsed.RawPath = ""
	}
	return parsed.String(), nil
}

func canonicalQuery(values url.Values) url.Values {
	result := make(url.Values, len(values))
	for key, entries := range values {
		lowerKey := strings.ToLower(key)
		if strings.HasPrefix(lowerKey, "utm_") || trackingQueryParameters[lowerKey] {
			continue
		}
		result[key] = append([]string(nil), entries...)
	}
	return result
}

// ClusterKey creates a stable identity for exact URL duplicates. A normalized
// title is used only when the source did not provide a usable URL.
func ClusterKey(rawURL, title string) string {
	if canonical, err := CanonicalURL(rawURL); err == nil {
		return hashIdentity("url", canonical)
	}
	normalizedTitle := normalizeTitle(title)
	if normalizedTitle == "" {
		return ""
	}
	return hashIdentity("title", normalizedTitle)
}

func normalizeTitle(title string) string {
	return strings.Join(normalizedTitleWords.FindAllString(strings.ToLower(clean(title)), -1), " ")
}

func hashIdentity(kind, value string) string {
	digest := sha256.Sum256([]byte(value))
	return kind + ":" + hex.EncodeToString(digest[:])
}
