package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const (
	// RefreshTokenLength is the number of random bytes for refresh tokens.
	RefreshTokenLength = 32
	// RefreshTokenPrefix identifies refresh tokens.
	RefreshTokenPrefix = "rt_"
)

// GenerateRefreshToken creates a new opaque refresh token.
// Returns the plaintext token (sent in cookie) and its SHA-256 hash (stored in PG).
func GenerateRefreshToken() (plaintext, hash string, err error) {
	buf := make([]byte, RefreshTokenLength)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}

	plaintext = RefreshTokenPrefix + hex.EncodeToString(buf)
	hash = HashRefreshToken(plaintext)
	return plaintext, hash, nil
}

// HashRefreshToken returns the SHA-256 hex digest of a refresh token.
func HashRefreshToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
