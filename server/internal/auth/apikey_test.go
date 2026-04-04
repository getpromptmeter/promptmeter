package auth

import (
	"strings"
	"testing"
)

func TestGenerateKey_LivePrefix(t *testing.T) {
	plaintext, hash, displayPrefix, err := GenerateKey(PrefixLive)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(plaintext, PrefixLive) {
		t.Errorf("plaintext should start with %q, got %q", PrefixLive, plaintext)
	}

	body := strings.TrimPrefix(plaintext, PrefixLive)
	if len(body) != keyLength {
		t.Errorf("body length should be %d, got %d", keyLength, len(body))
	}

	if hash == "" {
		t.Error("hash should not be empty")
	}

	if !strings.HasPrefix(displayPrefix, PrefixLive) {
		t.Errorf("display prefix should start with %q, got %q", PrefixLive, displayPrefix)
	}

	if len(displayPrefix) != len(PrefixLive)+4 {
		t.Errorf("display prefix should be %d chars, got %d", len(PrefixLive)+4, len(displayPrefix))
	}
}

func TestGenerateKey_TestPrefix(t *testing.T) {
	plaintext, _, _, err := GenerateKey(PrefixTest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(plaintext, PrefixTest) {
		t.Errorf("plaintext should start with %q, got %q", PrefixTest, plaintext)
	}
}

func TestGenerateKey_InvalidPrefix(t *testing.T) {
	_, _, _, err := GenerateKey("invalid_")
	if err == nil {
		t.Error("expected error for invalid prefix")
	}
}

func TestHashKey_Roundtrip(t *testing.T) {
	key := "pm_live_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdef"
	hash1 := HashKey(key)
	hash2 := HashKey(key)
	if hash1 != hash2 {
		t.Error("hashing the same key should produce the same hash")
	}
	if len(hash1) != 64 {
		t.Errorf("SHA-256 hex should be 64 chars, got %d", len(hash1))
	}
}

func TestHashKey_DifferentKeys(t *testing.T) {
	hash1 := HashKey("pm_live_AAAAAAAAAAAAAAAAAAAAAAAAAAAAaaaa")
	hash2 := HashKey("pm_live_BBBBBBBBBBBBBBBBBBBBBBBBBBBBbbbb")
	if hash1 == hash2 {
		t.Error("different keys should produce different hashes")
	}
}

func TestValidateFormat_Valid(t *testing.T) {
	plaintext, _, _, _ := GenerateKey(PrefixLive)
	if err := ValidateFormat(plaintext); err != nil {
		t.Errorf("valid key should pass: %v", err)
	}
}

func TestValidateFormat_InvalidPrefix(t *testing.T) {
	if err := ValidateFormat("xx_invalid_ABCDEFGHIJKLMNOPQRSTUVWXYZab"); err == nil {
		t.Error("should reject invalid prefix")
	}
}

func TestValidateFormat_ShortBody(t *testing.T) {
	if err := ValidateFormat("pm_live_short"); err == nil {
		t.Error("should reject short body")
	}
}

func TestValidateFormat_InvalidChars(t *testing.T) {
	if err := ValidateFormat("pm_live_ABCDEFGHIJKLMNOPQRSTUVWXYZ!@#$%^"); err == nil {
		t.Error("should reject non-base62 characters")
	}
}

func TestExtractPrefix(t *testing.T) {
	key := "pm_live_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdef"
	prefix := ExtractPrefix(key)
	if prefix != "pm_live_ABCD" {
		t.Errorf("expected %q, got %q", "pm_live_ABCD", prefix)
	}
}

func TestExtractPrefix_Test(t *testing.T) {
	key := "pm_test_XYZWabcdefghijklmnopqrstuvwxyz12"
	prefix := ExtractPrefix(key)
	if prefix != "pm_test_XYZW" {
		t.Errorf("expected %q, got %q", "pm_test_XYZW", prefix)
	}
}

func TestGenerateKey_Uniqueness(t *testing.T) {
	keys := make(map[string]bool)
	for i := 0; i < 100; i++ {
		plaintext, _, _, err := GenerateKey(PrefixLive)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if keys[plaintext] {
			t.Fatalf("duplicate key generated")
		}
		keys[plaintext] = true
	}
}
