package auth

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	telegramIssuer   = "https://oauth.telegram.org"
	telegramAuthURL  = "https://oauth.telegram.org/auth"
	telegramTokenURL = "https://oauth.telegram.org/token"
	telegramJWKSURL  = "https://oauth.telegram.org/.well-known/jwks.json"
)

type OIDCClient struct {
	clientID     string
	clientSecret string
	redirectURI  string
	http         *http.Client
	now          func() time.Time
	jwksMu       sync.Mutex
	jwks         map[string]*rsa.PublicKey
	jwksExpiry   time.Time
}

type Claims struct {
	Issuer            string          `json:"iss"`
	Audience          json.RawMessage `json:"aud"`
	Subject           string          `json:"sub"`
	TelegramUserID    int64           `json:"id"`
	ExpiresAt         int64           `json:"exp"`
	IssuedAt          int64           `json:"iat"`
	Nonce             string          `json:"nonce"`
	Name              string          `json:"name"`
	PreferredUsername string          `json:"preferred_username"`
	Picture           string          `json:"picture"`
}

func NewOIDCClient(clientID, clientSecret, redirectURI string, client *http.Client) *OIDCClient {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &OIDCClient{clientID: clientID, clientSecret: clientSecret, redirectURI: redirectURI,
		http: client, now: time.Now, jwks: make(map[string]*rsa.PublicKey)}
}

func (client *OIDCClient) AuthorizationURL(state, nonce, challenge string) string {
	query := url.Values{
		"client_id":             {client.clientID},
		"redirect_uri":          {client.redirectURI},
		"response_type":         {"code"},
		"scope":                 {"openid profile telegram:bot_access"},
		"state":                 {state},
		"nonce":                 {nonce},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	return telegramAuthURL + "?" + query.Encode()
}

func (client *OIDCClient) Exchange(ctx context.Context, code, verifier, expectedNonce string) (Claims, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {client.redirectURI},
		"code_verifier": {verifier},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, telegramTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return Claims{}, err
	}
	request.SetBasicAuth(client.clientID, client.clientSecret)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := client.http.Do(request)
	if err != nil {
		return Claims{}, fmt.Errorf("exchange Telegram authorization: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return Claims{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return Claims{}, fmt.Errorf("Telegram token exchange returned %s", response.Status)
	}
	var tokens struct {
		IDToken string `json:"id_token"`
	}
	if err := json.Unmarshal(body, &tokens); err != nil {
		return Claims{}, fmt.Errorf("decode Telegram token exchange: %w", err)
	}
	if tokens.IDToken == "" {
		return Claims{}, errors.New("Telegram response did not contain an ID token")
	}
	return client.VerifyIDToken(ctx, tokens.IDToken, expectedNonce)
}

func (client *OIDCClient) VerifyIDToken(ctx context.Context, token, expectedNonce string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, errors.New("invalid Telegram ID token")
	}
	var header struct {
		Algorithm string `json:"alg"`
		KeyID     string `json:"kid"`
	}
	if err := decodeSegment(parts[0], &header); err != nil {
		return Claims{}, errors.New("invalid Telegram ID token header")
	}
	if header.Algorithm != "RS256" {
		return Claims{}, fmt.Errorf("unsupported Telegram signing algorithm %q; configure RS256 in BotFather", header.Algorithm)
	}
	key, err := client.key(ctx, header.KeyID)
	if err != nil {
		return Claims{}, err
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Claims{}, errors.New("invalid Telegram ID token signature encoding")
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], signature); err != nil {
		return Claims{}, errors.New("Telegram ID token signature is invalid")
	}
	var claims Claims
	if err := decodeSegment(parts[1], &claims); err != nil {
		return Claims{}, errors.New("invalid Telegram ID token claims")
	}
	if claims.Issuer != telegramIssuer {
		return Claims{}, errors.New("Telegram ID token issuer is invalid")
	}
	if !audienceContains(claims.Audience, client.clientID) {
		return Claims{}, errors.New("Telegram ID token audience is invalid")
	}
	if claims.Subject == "" || claims.ExpiresAt <= client.now().Unix() {
		return Claims{}, errors.New("Telegram ID token is expired or missing a subject")
	}
	if claims.TelegramUserID <= 0 {
		return Claims{}, errors.New("Telegram ID token is missing its user ID")
	}
	if expectedNonce == "" || claims.Nonce != expectedNonce {
		return Claims{}, errors.New("Telegram ID token nonce is invalid")
	}
	return claims, nil
}

func (client *OIDCClient) key(ctx context.Context, keyID string) (*rsa.PublicKey, error) {
	client.jwksMu.Lock()
	defer client.jwksMu.Unlock()
	if key := client.jwks[keyID]; key != nil && client.jwksExpiry.After(client.now()) {
		return key, nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, telegramJWKSURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch Telegram signing keys: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Telegram signing keys returned %s", response.Status)
	}
	var document jwkDocument
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("decode Telegram signing keys: %w", err)
	}
	client.jwks = parseJWK(document)
	client.jwksExpiry = client.now().Add(6 * time.Hour)
	if key := client.jwks[keyID]; key != nil {
		return key, nil
	}
	return nil, errors.New("Telegram JWKS did not contain the requested RS256 key")
}

type jwkDocument struct {
	Keys []jwk `json:"keys"`
}
type jwk struct {
	KeyID     string `json:"kid"`
	Type      string `json:"kty"`
	Algorithm string `json:"alg"`
	Modulus   string `json:"n"`
	Exponent  string `json:"e"`
}

func parseJWK(document jwkDocument) map[string]*rsa.PublicKey {
	keys := make(map[string]*rsa.PublicKey)
	for _, item := range document.Keys {
		if item.Type != "RSA" || item.Algorithm != "RS256" {
			continue
		}
		modulusBytes, err := base64.RawURLEncoding.DecodeString(item.Modulus)
		if err != nil {
			continue
		}
		exponentBytes, err := base64.RawURLEncoding.DecodeString(item.Exponent)
		if err != nil {
			continue
		}
		exponent := 0
		for _, value := range exponentBytes {
			exponent = exponent<<8 + int(value)
		}
		if exponent <= 0 {
			continue
		}
		keys[item.KeyID] = &rsa.PublicKey{N: new(big.Int).SetBytes(modulusBytes), E: exponent}
	}
	return keys
}

func decodeSegment(segment string, destination any) error {
	decoded, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		return err
	}
	return json.Unmarshal(decoded, destination)
}

func audienceContains(raw json.RawMessage, expected string) bool {
	var single string
	if json.Unmarshal(raw, &single) == nil {
		return single == expected
	}
	var multiple []string
	if json.Unmarshal(raw, &multiple) != nil {
		return false
	}
	for _, value := range multiple {
		if value == expected {
			return true
		}
	}
	return false
}

func PKCEChallenge(verifier string) string {
	digest := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}

func NumericSubject(subject string) bool {
	_, err := strconv.ParseInt(subject, 10, 64)
	return err == nil
}
