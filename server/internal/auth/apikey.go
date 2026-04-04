// Package auth provides API key generation, hashing, and validation.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	// PrefixLive is the prefix for production API keys.
	PrefixLive = "pm_live_"
	// PrefixTest is the prefix for test/development API keys.
	PrefixTest = "pm_test_"
	// keyLength is the number of random bytes in the key body (base62-encoded).
	keyLength = 32
)

// base62Chars is the character set for base62 encoding.
const base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// GenerateKey creates a new API key with the given prefix ("pm_live_" or "pm_test_").
// Returns the full plaintext key (shown once) and its SHA-256 hash (stored).
func GenerateKey(prefix string) (plaintext string, hash string, displayPrefix string, err error) {
	if prefix != PrefixLive && prefix != PrefixTest {
		return "", "", "", fmt.Errorf("invalid prefix: must be %q or %q", PrefixLive, PrefixTest)
	}

	body, err := randomBase62(keyLength)
	if err != nil {
		return "", "", "", fmt.Errorf("generate key: %w", err)
	}

	plaintext = prefix + body
	hash = HashKey(plaintext)
	displayPrefix = prefix + body[:4]
	return plaintext, hash, displayPrefix, nil
}

// HashKey returns the SHA-256 hex digest of an API key.
func HashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

// ValidateFormat checks that a key string has the correct prefix and length.
func ValidateFormat(key string) error {
	if !strings.HasPrefix(key, PrefixLive) && !strings.HasPrefix(key, PrefixTest) {
		return fmt.Errorf("invalid api key format: must start with %q or %q", PrefixLive, PrefixTest)
	}

	var body string
	if strings.HasPrefix(key, PrefixLive) {
		body = strings.TrimPrefix(key, PrefixLive)
	} else {
		body = strings.TrimPrefix(key, PrefixTest)
	}

	if len(body) != keyLength {
		return fmt.Errorf("invalid api key format: body must be %d characters", keyLength)
	}

	for _, c := range body {
		if !strings.ContainsRune(base62Chars, c) {
			return fmt.Errorf("invalid api key format: body must be base62")
		}
	}

	return nil
}

// ExtractPrefix returns the display prefix (e.g., "pm_live_7kBx") from a full key.
func ExtractPrefix(key string) string {
	if strings.HasPrefix(key, PrefixLive) {
		body := strings.TrimPrefix(key, PrefixLive)
		if len(body) >= 4 {
			return PrefixLive + body[:4]
		}
	}
	if strings.HasPrefix(key, PrefixTest) {
		body := strings.TrimPrefix(key, PrefixTest)
		if len(body) >= 4 {
			return PrefixTest + body[:4]
		}
	}
	return ""
}

func randomBase62(n int) (string, error) {
	buf := make([]byte, n)
	_, err := rand.Read(buf)
	if err != nil {
		return "", err
	}
	result := make([]byte, n)
	for i := 0; i < n; i++ {
		result[i] = base62Chars[int(buf[i])%len(base62Chars)]
	}
	return string(result), nil
}
