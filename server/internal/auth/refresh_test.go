package auth

import (
	"strings"
	"testing"
)

func TestGenerateRefreshToken(t *testing.T) {
	plaintext, hash, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}

	if !strings.HasPrefix(plaintext, RefreshTokenPrefix) {
		t.Errorf("plaintext prefix = %q, want %q", plaintext[:3], RefreshTokenPrefix)
	}

	// 32 bytes = 64 hex chars + prefix
	expectedLen := len(RefreshTokenPrefix) + 64
	if len(plaintext) != expectedLen {
		t.Errorf("plaintext length = %d, want %d", len(plaintext), expectedLen)
	}

	if hash == "" {
		t.Error("hash is empty")
	}

	// Verify hash is deterministic
	if hash != HashRefreshToken(plaintext) {
		t.Error("hash mismatch on re-hash")
	}
}

func TestGenerateRefreshToken_Uniqueness(t *testing.T) {
	pt1, _, _ := GenerateRefreshToken()
	pt2, _, _ := GenerateRefreshToken()
	if pt1 == pt2 {
		t.Error("two generated tokens are identical")
	}
}
