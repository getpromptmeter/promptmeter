package auth

import (
	"testing"
	"time"
)

func TestSignAndVerifyJWT(t *testing.T) {
	secret := "test-secret-key"
	claims := JWTClaims{
		Sub:        "usr_123",
		Org:        "org_456",
		OrgNumeric: 12345,
		Role:       "owner",
		Tier:       "free",
		Iat:        time.Now().Unix(),
		Exp:        time.Now().Add(15 * time.Minute).Unix(),
	}

	token, err := SignJWT(claims, secret)
	if err != nil {
		t.Fatalf("SignJWT: %v", err)
	}

	got, err := VerifyJWT(token, secret)
	if err != nil {
		t.Fatalf("VerifyJWT: %v", err)
	}

	if got.Sub != claims.Sub {
		t.Errorf("Sub = %q, want %q", got.Sub, claims.Sub)
	}
	if got.Org != claims.Org {
		t.Errorf("Org = %q, want %q", got.Org, claims.Org)
	}
	if got.OrgNumeric != claims.OrgNumeric {
		t.Errorf("OrgNumeric = %d, want %d", got.OrgNumeric, claims.OrgNumeric)
	}
	if got.Role != claims.Role {
		t.Errorf("Role = %q, want %q", got.Role, claims.Role)
	}
	if got.Tier != claims.Tier {
		t.Errorf("Tier = %q, want %q", got.Tier, claims.Tier)
	}
}

func TestVerifyJWT_Expired(t *testing.T) {
	secret := "test-secret-key"
	claims := JWTClaims{
		Sub:        "usr_123",
		Org:        "org_456",
		OrgNumeric: 12345,
		Role:       "owner",
		Tier:       "free",
		Iat:        time.Now().Add(-1 * time.Hour).Unix(),
		Exp:        time.Now().Add(-30 * time.Minute).Unix(),
	}

	token, err := SignJWT(claims, secret)
	if err != nil {
		t.Fatalf("SignJWT: %v", err)
	}

	_, err = VerifyJWT(token, secret)
	if err != ErrTokenExpired {
		t.Errorf("VerifyJWT err = %v, want ErrTokenExpired", err)
	}
}

func TestVerifyJWT_TamperedSignature(t *testing.T) {
	secret := "test-secret-key"
	claims := JWTClaims{
		Sub:        "usr_123",
		Org:        "org_456",
		OrgNumeric: 12345,
		Role:       "owner",
		Tier:       "free",
	}

	token, err := SignJWT(claims, secret)
	if err != nil {
		t.Fatalf("SignJWT: %v", err)
	}

	_, err = VerifyJWT(token, "wrong-secret")
	if err != ErrSignatureInvalid {
		t.Errorf("VerifyJWT err = %v, want ErrSignatureInvalid", err)
	}
}

func TestVerifyJWT_Malformed(t *testing.T) {
	_, err := VerifyJWT("not-a-jwt", "secret")
	if err != ErrTokenMalformed {
		t.Errorf("VerifyJWT err = %v, want ErrTokenMalformed", err)
	}
}

func TestVerifyJWT_MissingClaims(t *testing.T) {
	secret := "test-secret"
	claims := JWTClaims{
		Sub: "usr_123",
		// Missing Org
	}
	token, _ := SignJWT(claims, secret)
	_, err := VerifyJWT(token, secret)
	if err != ErrTokenInvalid {
		t.Errorf("VerifyJWT err = %v, want ErrTokenInvalid", err)
	}
}
