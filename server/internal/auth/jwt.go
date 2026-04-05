package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// JWTClaims represents the claims in a Promptmeter JWT.
type JWTClaims struct {
	Sub        string `json:"sub"`         // user_id
	Org        string `json:"org"`         // org_id (UUID string)
	OrgNumeric uint64 `json:"org_numeric"` // org_id as uint64 for ClickHouse
	Role       string `json:"role"`        // "owner", "member"
	Tier       string `json:"tier"`        // "free", "pro", "business", "enterprise"
	Iat        int64  `json:"iat"`         // issued at
	Exp        int64  `json:"exp"`         // expiration
}

var (
	ErrTokenExpired     = errors.New("token expired")
	ErrTokenInvalid     = errors.New("token invalid")
	ErrTokenMalformed   = errors.New("token malformed")
	ErrSignatureInvalid = errors.New("signature invalid")
)

const (
	// DefaultJWTExpiry is the default JWT expiration time.
	DefaultJWTExpiry = 15 * time.Minute
)

// jwtHeader is the fixed header for HS256 JWTs.
var jwtHeaderB64 = base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))

// SignJWT creates a signed JWT with the given claims and secret.
func SignJWT(claims JWTClaims, secret string) (string, error) {
	if claims.Iat == 0 {
		claims.Iat = time.Now().Unix()
	}
	if claims.Exp == 0 {
		claims.Exp = time.Now().Add(DefaultJWTExpiry).Unix()
	}

	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("jwt: marshal claims: %w", err)
	}

	payloadB64 := base64.RawURLEncoding.EncodeToString(payload)
	signingInput := jwtHeaderB64 + "." + payloadB64

	sig := signHS256(signingInput, secret)
	return signingInput + "." + sig, nil
}

// VerifyJWT parses and verifies a JWT, returning the claims.
func VerifyJWT(token, secret string) (*JWTClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrTokenMalformed
	}

	signingInput := parts[0] + "." + parts[1]
	expectedSig := signHS256(signingInput, secret)

	if !hmac.Equal([]byte(parts[2]), []byte(expectedSig)) {
		return nil, ErrSignatureInvalid
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrTokenMalformed
	}

	var claims JWTClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, ErrTokenMalformed
	}

	if claims.Sub == "" || claims.Org == "" {
		return nil, ErrTokenInvalid
	}

	if time.Now().Unix() > claims.Exp {
		return nil, ErrTokenExpired
	}

	return &claims, nil
}

func signHS256(input, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(input))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
