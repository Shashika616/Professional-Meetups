package jwt

import (
	"crypto/rsa"
	"fmt"
	"os"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

// AccessTokenTTL is the lifetime of an issued access token (ADR-009). This
// is a security parameter, not a deployment parameter — it is a hardcoded
// constant, deliberately not read from the environment.
const AccessTokenTTL = 15 * time.Minute

const issuer = "professional-connections-auth"

// Signer issues signed access tokens. Per ADR-009, only the auth service
// holds the private key and constructs a Signer; every other service holds
// only the public key via a Verifier (see verifier.go).
type Signer struct {
	privateKey *rsa.PrivateKey
}

// NewSigner loads an RSA private key from privateKeyPath and fails fast if
// it can't be read or parsed — a key that won't parse should crash the
// process at startup, not surface as an error on the first request that
// needs to sign a token.
func NewSigner(privateKeyPath string) (*Signer, error) {
	raw, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("jwt: read private key %q: %w", privateKeyPath, err)
	}

	key, err := jwtlib.ParseRSAPrivateKeyFromPEM(raw)
	if err != nil {
		return nil, fmt.Errorf("jwt: parse private key %q: %w", privateKeyPath, err)
	}

	return &Signer{privateKey: key}, nil
}

// Sign issues a signed access token for claims. IssuedAt, ExpiresAt, and
// Issuer are set here and override anything the caller supplied — callers
// are only responsible for UserID, TrustLevel, and optionally Subject.
func (s *Signer) Sign(claims Claims) (string, error) {
	now := time.Now()
	claims.IssuedAt = jwtlib.NewNumericDate(now)
	claims.ExpiresAt = jwtlib.NewNumericDate(now.Add(AccessTokenTTL))
	claims.Issuer = issuer
	if claims.Subject == "" {
		claims.Subject = claims.UserID
	}

	token := jwtlib.NewWithClaims(jwtlib.SigningMethodRS256, claims)
	signed, err := token.SignedString(s.privateKey)
	if err != nil {
		return "", fmt.Errorf("jwt: sign: %w", err)
	}
	return signed, nil
}
