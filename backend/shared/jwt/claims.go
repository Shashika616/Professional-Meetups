// Package jwt implements RS256 signing and verification of Professional
// Connections session tokens (ADR-009). Only the auth service constructs a
// Signer; every other service constructs a Verifier — see signer.go and
// verifier.go.
package jwt

import (
	jwtlib "github.com/golang-jwt/jwt/v5"
)

// Claims are the JWT claims issued for an authenticated session. UserID and
// TrustLevel are this project's own claims; RegisteredClaims carries the
// standard exp/iat/iss/sub claims (RFC 7519).
type Claims struct {
	UserID     string `json:"user_id"`
	TrustLevel int    `json:"trust_level"`
	jwtlib.RegisteredClaims
}
