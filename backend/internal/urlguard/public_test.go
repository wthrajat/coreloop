package urlguard

import "testing"

func TestSafeExternalHTTPSURL(t *testing.T) {
	for _, value := range []string{
		"http://go.dev/blog/source",
		"https://localhost/source",
		"https://service.internal/source",
		"https://printer.lan/source",
		"https://127.0.0.1/source",
		"https://10.0.0.1/source",
		"https://[::1]/source",
		"https://192.0.2.1/source",
		"https://user:secret@go.dev/source",
		"https://go.dev:8443/source",
		"https://example.com/source",
	} {
		if IsSafeExternalHTTPSURL(value) {
			t.Fatalf("unsafe URL %q was accepted", value)
		}
	}
	if !IsSafeExternalHTTPSURL("https://go.dev:443/blog/source") {
		t.Fatal("public HTTPS URL was rejected")
	}
	if !IsSafeExternalHTTPSURL("https://[2606:4700:4700::1111]/source") {
		t.Fatal("public IPv6 HTTPS URL was rejected")
	}
}
