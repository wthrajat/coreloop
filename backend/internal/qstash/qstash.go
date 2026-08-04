package qstash

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Receiver struct {
	current, next string
	now           func() time.Time
}

func NewReceiver(current, next string) *Receiver {
	return &Receiver{current: current, next: next, now: time.Now}
}

func (receiver *Receiver) Verify(signature, expectedURL string, body []byte) error {
	if signature == "" {
		return errors.New("missing Upstash-Signature")
	}
	parts := strings.Split(signature, ".")
	if len(parts) != 3 {
		return errors.New("invalid QStash signature")
	}
	var claims struct {
		Issuer    string `json:"iss"`
		Subject   string `json:"sub"`
		Expires   int64  `json:"exp"`
		NotBefore int64  `json:"nbf"`
		Body      string `json:"body"`
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return errors.New("invalid QStash claims")
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return errors.New("invalid QStash claims")
	}
	valid := verifySignature(parts, receiver.current) || verifySignature(parts, receiver.next)
	if !valid {
		return errors.New("QStash signature does not match")
	}
	now := receiver.now().Unix()
	if claims.Issuer != "Upstash" || claims.Subject != expectedURL || claims.Expires < now || claims.NotBefore > now+30 {
		return errors.New("QStash claims are invalid")
	}
	digest := sha256.Sum256(body)
	expectedBody := base64.RawURLEncoding.EncodeToString(digest[:])
	if !hmac.Equal([]byte(expectedBody), []byte(claims.Body)) {
		return errors.New("QStash body hash is invalid")
	}
	return nil
}

func verifySignature(parts []string, key string) bool {
	if key == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(parts[2]))
}

type Publisher struct {
	token   string
	client  *http.Client
	baseURL string
}

func NewPublisher(token string, client *http.Client) *Publisher {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &Publisher{token: token, client: client, baseURL: "https://qstash.upstash.io"}
}

func (publisher *Publisher) Publish(ctx context.Context, destination, deduplicationID string, payload any) error {
	if publisher.token == "" {
		return errors.New("QStash token is not configured")
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	endpoint := publisher.baseURL + "/v2/publish/" + url.PathEscape(destination)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+publisher.token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Upstash-Deduplication-Id", deduplicationID)
	request.Header.Set("Upstash-Retries", "3")
	response, err := publisher.client.Do(request)
	if err != nil {
		return fmt.Errorf("publish QStash job: %w", err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("QStash publish returned %s: %.400s", response.Status, body)
	}
	return nil
}
