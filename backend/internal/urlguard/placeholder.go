package urlguard

import "strings"

var reservedHosts = map[string]bool{
	"example":     true,
	"example.com": true,
	"example.net": true,
	"example.org": true,
	"invalid":     true,
	"localhost":   true,
	"test":        true,
}

// IsPlaceholderHost identifies RFC-reserved hosts that may appear in examples
// and model output but can never be a real content source.
func IsPlaceholderHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if reservedHosts[host] {
		return true
	}
	for _, suffix := range []string{
		".example",
		".example.com",
		".example.net",
		".example.org",
		".invalid",
		".localhost",
		".test",
	} {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}
	return false
}
