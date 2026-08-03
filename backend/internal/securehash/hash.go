package securehash

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
)

func SHA256(value string) string {
	sum := sha256.Sum256([]byte(value))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func Keyed(value, secret string) string {
	hasher := hmac.New(sha256.New, []byte(secret))
	_, _ = hasher.Write([]byte(value))
	return base64.RawURLEncoding.EncodeToString(hasher.Sum(nil))
}

func Equal(left, right string) bool { return hmac.Equal([]byte(left), []byte(right)) }
