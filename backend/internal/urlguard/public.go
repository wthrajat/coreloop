package urlguard

import (
	"net"
	"net/netip"
	"net/url"
	"strings"
)

var nonPublicPrefixes = []netip.Prefix{
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("2001:db8::/32"),
}

// IsPublicWebHost rejects local, reserved, and example-only destinations. DNS
// is intentionally not resolved here; server-side fetches must also validate
// every resolved address at connection time to prevent DNS rebinding.
func IsPublicWebHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if host == "" || IsPlaceholderHost(host) {
		return false
	}
	for _, suffix := range []string{".internal", ".lan", ".local", ".home.arpa"} {
		if strings.HasSuffix(host, suffix) {
			return false
		}
	}
	address := net.ParseIP(host)
	if address == nil {
		return strings.Contains(host, ".")
	}
	if !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() ||
		address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() ||
		address.IsUnspecified() {
		return false
	}
	parsed, ok := netip.AddrFromSlice(address)
	if !ok {
		return false
	}
	parsed = parsed.Unmap()
	for _, blocked := range nonPublicPrefixes {
		if blocked.Contains(parsed) {
			return false
		}
	}
	return true
}

func IsSafeExternalHTTPSURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return false
	}
	if port := parsed.Port(); port != "" && port != "443" {
		return false
	}
	return IsPublicWebHost(parsed.Hostname())
}
