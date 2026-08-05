package auth

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestAuthorizationURLUsesPKCEAndBotAccess(t *testing.T) {
	client := NewOIDCClient("123", "secret", "https://example.com/api/app/auth/callback", nil)
	parsed, err := url.Parse(client.AuthorizationURL("state", "nonce", PKCEChallenge("verifier")))
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	if query.Get("code_challenge_method") != "S256" {
		t.Fatal("PKCE method missing")
	}
	if query.Get("scope") != "openid profile telegram:bot_access" {
		t.Fatalf("scope=%q", query.Get("scope"))
	}
}

func TestAudienceContainsStringOrArray(t *testing.T) {
	if !audienceContains(json.RawMessage(`"client"`), "client") {
		t.Fatal("single audience rejected")
	}
	if !audienceContains(json.RawMessage(`["other","client"]`), "client") {
		t.Fatal("array audience rejected")
	}
}

func TestVerifyTelegramIDTokenChecksRS256Claims(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	exponent := bigEndian(privateKey.PublicKey.E)
	jwks, _ := json.Marshal(map[string]any{"keys": []map[string]string{{"kid": "key-1", "kty": "RSA", "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(privateKey.PublicKey.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(exponent)}}})
	httpClient := &http.Client{Transport: authRoundTrip(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(jwks))}, nil
	})}
	client := NewOIDCClient("client-1", "secret", "https://example.com/callback", httpClient)
	client.now = func() time.Time { return time.Unix(1_800_000_000, 0) }
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "key-1"})
	claims, _ := json.Marshal(map[string]any{"iss": telegramIssuer, "aud": "client-1", "sub": "opaque-subject", "id": int64(12345), "exp": int64(1_800_000_300), "nonce": "expected"})
	encodedHeader := base64.RawURLEncoding.EncodeToString(header)
	encodedClaims := base64.RawURLEncoding.EncodeToString(claims)
	digest := sha256.Sum256([]byte(encodedHeader + "." + encodedClaims))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	token := encodedHeader + "." + encodedClaims + "." + base64.RawURLEncoding.EncodeToString(signature)
	verified, err := client.VerifyIDToken(context.Background(), token, "expected")
	if err != nil {
		t.Fatal(err)
	}
	if verified.Subject != "opaque-subject" || verified.TelegramUserID != 12345 {
		t.Fatalf("claims=%#v", verified)
	}
}

func TestIdentityUsesTelegramUserIDForBotDelivery(t *testing.T) {
	identity := identityFromClaims(Claims{
		Subject: "opaque-authentication-subject", TelegramUserID: 987654321,
		Name: "Coreloop Owner", PreferredUsername: "owner",
	})
	if identity.Subject != "opaque-authentication-subject" {
		t.Fatalf("subject = %q", identity.Subject)
	}
	if identity.TelegramChatID != "987654321" {
		t.Fatalf("Telegram chat ID = %q", identity.TelegramChatID)
	}
}

func TestCallbackRequiresTheStartingBrowserBinding(t *testing.T) {
	service := &Service{}
	if _, err := service.Callback(
		context.Background(),
		"authorization-code",
		"state",
		"",
	); err == nil {
		t.Fatal("callback accepted a missing browser binding")
	}
}

func TestVerifyTelegramIDTokenRejectsInvalidAuthorizedPartyAndIssueTime(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	client := oidcTestClient(t, privateKey)
	now := time.Unix(1_800_000_000, 0)
	client.now = func() time.Time { return now }
	baseClaims := map[string]any{
		"iss":   telegramIssuer,
		"aud":   "client-1",
		"sub":   "12345",
		"id":    int64(12345),
		"exp":   now.Add(5 * time.Minute).Unix(),
		"iat":   now.Unix(),
		"nonce": "expected",
	}

	for name, mutate := range map[string]func(map[string]any){
		"mismatched authorized party": func(claims map[string]any) {
			claims["azp"] = "other-client"
		},
		"missing authorized party for multiple audiences": func(claims map[string]any) {
			claims["aud"] = []string{"client-1", "other-client"}
		},
		"future issue time": func(claims map[string]any) {
			claims["iat"] = now.Add(3 * time.Minute).Unix()
		},
	} {
		t.Run(name, func(t *testing.T) {
			claims := make(map[string]any, len(baseClaims))
			for key, value := range baseClaims {
				claims[key] = value
			}
			mutate(claims)
			if _, err := client.VerifyIDToken(
				context.Background(),
				signedOIDCTestToken(t, privateKey, claims),
				"expected",
			); err == nil {
				t.Fatal("invalid ID token was accepted")
			}
		})
	}
}

type authRoundTrip func(*http.Request) (*http.Response, error)

func (function authRoundTrip) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
func bigEndian(value int) []byte {
	encoded := []byte{}
	for value > 0 {
		encoded = append([]byte{byte(value)}, encoded...)
		value >>= 8
	}
	if len(encoded) == 0 {
		return []byte{0}
	}
	return encoded
}

func oidcTestClient(t *testing.T, privateKey *rsa.PrivateKey) *OIDCClient {
	t.Helper()
	jwks, err := json.Marshal(map[string]any{"keys": []map[string]string{{
		"kid": "key-1", "kty": "RSA", "alg": "RS256",
		"n": base64.RawURLEncoding.EncodeToString(privateKey.PublicKey.N.Bytes()),
		"e": base64.RawURLEncoding.EncodeToString(bigEndian(privateKey.PublicKey.E)),
	}}})
	if err != nil {
		t.Fatal(err)
	}
	httpClient := &http.Client{Transport: authRoundTrip(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(jwks)),
		}, nil
	})}
	return NewOIDCClient("client-1", "secret", "https://example.com/callback", httpClient)
}

func signedOIDCTestToken(
	t *testing.T,
	privateKey *rsa.PrivateKey,
	claims map[string]any,
) string {
	t.Helper()
	header, err := json.Marshal(map[string]string{"alg": "RS256", "kid": "key-1"})
	if err != nil {
		t.Fatal(err)
	}
	claimPayload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	encodedHeader := base64.RawURLEncoding.EncodeToString(header)
	encodedClaims := base64.RawURLEncoding.EncodeToString(claimPayload)
	digest := sha256.Sum256([]byte(encodedHeader + "." + encodedClaims))
	signature, err := rsa.SignPKCS1v15(
		rand.Reader,
		privateKey,
		crypto.SHA256,
		digest[:],
	)
	if err != nil {
		t.Fatal(err)
	}
	return encodedHeader + "." + encodedClaims + "." +
		base64.RawURLEncoding.EncodeToString(signature)
}
