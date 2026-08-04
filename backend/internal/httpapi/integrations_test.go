package httpapi

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"coreloop/backend/internal/qstash"
)

type blockingJobRunner struct {
	deadline time.Time
}

func (*blockingJobRunner) Tick(context.Context) error { return nil }

func (runner *blockingJobRunner) Run(ctx context.Context, _, _ string) error {
	runner.deadline, _ = ctx.Deadline()
	<-ctx.Done()
	return ctx.Err()
}

func TestJobsRunStopsBeforeTheFunctionDeadline(t *testing.T) {
	key := "test-qstash-signing-key"
	body := []byte(`{"job_id":"job_slow"}`)
	runner := &blockingJobRunner{}
	handler := NewJobsRouter(JobsConfig{
		AppOrigin:  "https://coreloop.example",
		Receiver:   qstash.NewReceiver(key, ""),
		Jobs:       runner,
		RunTimeout: 20 * time.Millisecond,
	})
	request := httptest.NewRequest(http.MethodPost, "/api/jobs/run", bytes.NewReader(body))
	request.Header.Set("Upstash-Signature", signedQStashRequest(t, key, "https://coreloop.example/api/jobs/run", body))
	response := httptest.NewRecorder()

	started := time.Now()
	handler.ServeHTTP(response, request)
	elapsed := time.Since(started)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("response status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	if runner.deadline.IsZero() {
		t.Fatal("job runner did not receive an invocation deadline")
	}
	if elapsed > 250*time.Millisecond {
		t.Fatalf("timed job handler took %s", elapsed)
	}
}

func signedQStashRequest(t *testing.T, key, subject string, body []byte) string {
	t.Helper()
	digest := sha256.Sum256(body)
	header, err := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := json.Marshal(map[string]any{
		"iss":  "Upstash",
		"sub":  subject,
		"exp":  time.Now().Add(time.Minute).Unix(),
		"nbf":  time.Now().Add(-time.Minute).Unix(),
		"body": base64.RawURLEncoding.EncodeToString(digest[:]),
	})
	if err != nil {
		t.Fatal(err)
	}
	parts := []string{
		base64.RawURLEncoding.EncodeToString(header),
		base64.RawURLEncoding.EncodeToString(claims),
	}
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	return parts[0] + "." + parts[1] + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
