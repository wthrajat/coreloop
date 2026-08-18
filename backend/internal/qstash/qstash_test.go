package qstash

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestReceiverVerifiesSignatureSubjectAndBody(t *testing.T) {
	key := "c2VjcmV0-with-a-qstash-shaped-key"
	body := []byte(`{"job_id":"job_1"}`)
	digest := sha256.Sum256(body)
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	claims, _ := json.Marshal(map[string]any{"iss": "Upstash", "sub": "https://example.com/api/jobs/run", "exp": time.Now().Add(time.Minute).Unix(), "nbf": time.Now().Add(-time.Minute).Unix(), "body": base64.URLEncoding.EncodeToString(digest[:])})
	parts := []string{base64.RawURLEncoding.EncodeToString(header), base64.RawURLEncoding.EncodeToString(claims)}
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	token := parts[0] + "." + parts[1] + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if err := NewReceiver(key, "").Verify(token, "https://example.com/api/jobs/run", body); err != nil {
		t.Fatal(err)
	}
}

func TestReceiverRejectsAFalseJWTAlgorithmEvenWithAValidMAC(t *testing.T) {
	key := "c2VjcmV0-with-a-qstash-shaped-key"
	body := []byte(`{"job_id":"job_1"}`)
	digest := sha256.Sum256(body)
	header, _ := json.Marshal(map[string]string{"alg": "none", "typ": "JWT"})
	claims, _ := json.Marshal(map[string]any{
		"iss": "Upstash", "sub": "https://example.com/api/jobs/run",
		"exp":  time.Now().Add(time.Minute).Unix(),
		"nbf":  time.Now().Add(-time.Minute).Unix(),
		"body": base64.RawURLEncoding.EncodeToString(digest[:]),
	})
	parts := []string{
		base64.RawURLEncoding.EncodeToString(header),
		base64.RawURLEncoding.EncodeToString(claims),
	}
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	token := parts[0] + "." + parts[1] + "." +
		base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	if err := NewReceiver(key, "").Verify(
		token,
		"https://example.com/api/jobs/run",
		body,
	); err == nil {
		t.Fatal("receiver accepted a token with a false algorithm header")
	}
}

func TestPublisherPreservesDestinationURLInPublishPath(t *testing.T) {
	type capturedRequest struct {
		escapedPath     string
		authorization   string
		deduplicationID string
		body            string
	}
	var captured capturedRequest
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		captured = capturedRequest{
			escapedPath:     request.URL.EscapedPath(),
			authorization:   request.Header.Get("Authorization"),
			deduplicationID: request.Header.Get("Upstash-Deduplication-Id"),
			body:            string(body),
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
			Request:    request,
		}, nil
	})}

	publisher := NewPublisher("qstash-token", client)
	publisher.baseURL = "https://qstash.upstash.io"
	destination := "https://coreloop.example/api/jobs/run"
	if err := publisher.Publish(context.Background(), destination, "dispatch-job_1", map[string]string{"job_id": "job_1"}); err != nil {
		t.Fatal(err)
	}

	if captured.escapedPath != "/v2/publish/https://coreloop.example/api/jobs/run" {
		t.Fatalf("unexpected escaped path: %q", captured.escapedPath)
	}
	if captured.authorization != "Bearer qstash-token" {
		t.Fatalf("unexpected authorization header: %q", captured.authorization)
	}
	if captured.deduplicationID != "dispatch-job_1" {
		t.Fatalf("unexpected deduplication ID: %q", captured.deduplicationID)
	}
	if captured.body != `{"job_id":"job_1"}` {
		t.Fatalf("unexpected body: %q", captured.body)
	}
}
