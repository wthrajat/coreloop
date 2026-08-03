package qstash

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func TestReceiverVerifiesSignatureSubjectAndBody(t *testing.T) {
	key := base64.RawURLEncoding.EncodeToString([]byte("secret"))
	body := []byte(`{"job_id":"job_1"}`)
	digest := sha256.Sum256(body)
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	claims, _ := json.Marshal(map[string]any{"iss": "Upstash", "sub": "https://example.com/api/jobs/run", "exp": time.Now().Add(time.Minute).Unix(), "nbf": time.Now().Add(-time.Minute).Unix(), "body": base64.RawURLEncoding.EncodeToString(digest[:])})
	parts := []string{base64.RawURLEncoding.EncodeToString(header), base64.RawURLEncoding.EncodeToString(claims)}
	mac := hmac.New(sha256.New, []byte("secret"))
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	token := parts[0] + "." + parts[1] + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if err := NewReceiver(key, "").Verify(token, "https://example.com/api/jobs/run", body); err != nil {
		t.Fatal(err)
	}
}
