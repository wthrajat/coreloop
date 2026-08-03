package ids

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

func New(prefix string) (string, error) {
	bytes := make([]byte, 18)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return prefix + "_" + base64.RawURLEncoding.EncodeToString(bytes), nil
}

func Token(byteCount int) (string, error) {
	if byteCount < 16 {
		byteCount = 16
	}
	bytes := make([]byte, byteCount)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
