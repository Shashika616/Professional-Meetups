// Package identity verifies federated id_tokens handed to the mobile app
// directly by Apple's and Google's native sign-in SDKs (ADR-014). This is a
// materially different flow from internal/linkedin's authorization-code
// exchange: there is no server-to-server code exchange here, and no
// client_secret — verification is purely cryptographic, checking the
// token's signature against each provider's own published JSON Web Key Set
// (JWKS), plus its issuer, audience, and expiry. Don't try to force this
// into linkedin/client.go's shape.
package identity

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/MicahParks/keyfunc/v3"
	jwtlib "github.com/golang-jwt/jwt/v5"
)

// defaultTimeout bounds every JWKS HTTP fetch (both the initial one at
// construction and every periodic refresh) — same defaultTimeout =
// 5 * time.Second pattern as linkedin/client.go and fcm.go; the zero-value
// http.Client has no timeout at all, which would let a hung request block
// indefinitely.
const defaultTimeout = 5 * time.Second

// jwksRefreshInterval is how often a provider's JWKS is re-fetched in the
// background after the initial synchronous fetch — a deliberately simple
// fixed interval rather than parsing each response's Cache-Control header:
// Apple/Google both keep multiple active signing keys published
// simultaneously through a rotation, so an hourly poll comfortably beats
// any real rotation window without needing to special-case key-not-found
// into an immediate out-of-band refresh.
const jwksRefreshInterval = time.Hour

// VerifiedIdentity is what a successful Provider.Verify call yields — the
// caller's stable subject id plus best-effort display fields. Name is
// often "" on Apple/Google id_tokens (Apple in particular only includes it
// on a user's very first authorization with this app, never again after)
// — unlike LinkedIn's userinfo endpoint, which always returns one, callers
// here must handle "" gracefully.
type VerifiedIdentity struct {
	Subject string
	Email   string
	Name    string
}

// Provider verifies a native-SDK-issued id_token against one federated
// identity provider's own published keys.
type Provider interface {
	Verify(ctx context.Context, idToken string) (VerifiedIdentity, error)
}

// idTokenClaims is the subset of an Apple/Google id_token this package
// needs. jwtlib.RegisteredClaims already carries iss/sub/aud/exp (RFC
// 7519's registered claim names); email/name are provider-specific public
// claims layered on top, present on both Apple's and Google's id_tokens
// under these exact JSON keys.
type idTokenClaims struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	jwtlib.RegisteredClaims
}

// jwksProvider is the shared verification engine behind AppleProvider and
// GoogleProvider — both are "RSA-signed id_token, verified against a
// published JWKS, with an issuer allowlist and a single expected
// audience," differing only in which URL/issuers/audience apply. Kept as
// one implementation rather than duplicated per provider.
type jwksProvider struct {
	name         string // "apple" / "google" — error messages only
	keyfunc      keyfunc.Keyfunc
	validIssuers []string
	audience     string
}

// newJWKSProvider attempts to fetch jwksURL once, synchronously, before
// returning, then refreshes every jwksRefreshInterval in a background
// goroutine tied to ctx (confirmed by reading jwkset.NewStorageFromHTTP's
// actual source, not just its doc comment — the doc comment reads as if
// setting RefreshInterval skips the synchronous first fetch; it doesn't).
//
// A failed first fetch does NOT fail construction (NoErrorReturnFirstHTTPReq)
// — deliberately not the fail-fast-at-startup treatment cmd/server/main.go
// gives pool.Ping. jwksURL is always one of this package's own hardcoded
// constants, never derived from configuration, so a failure here can only
// ever mean "Apple/Google's endpoint is transiently unreachable," never "a
// human typo'd a URL" — and letting that transient failure crash the
// entire auth service (LinkedIn/phone/email verification included, none
// of which has anything to do with Apple/Google) is a worse outcome than
// this provider simply rejecting every token until a later background
// refresh succeeds. Errors from every fetch attempt (first and
// subsequent) are still logged via RefreshErrorHandler, so a persistently
// unreachable endpoint isn't silent.
func newJWKSProvider(ctx context.Context, name, jwksURL, audience string, validIssuers []string) (*jwksProvider, error) {
	httpClient := &http.Client{Timeout: defaultTimeout}

	storage, err := jwkset.NewStorageFromHTTP(jwksURL, jwkset.HTTPClientStorageOptions{
		Client:                    httpClient,
		Ctx:                       ctx,
		HTTPTimeout:               defaultTimeout,
		RefreshInterval:           jwksRefreshInterval,
		NoErrorReturnFirstHTTPReq: true,
		RefreshErrorHandler: func(_ context.Context, refreshErr error) {
			slog.Default().Error("identity: JWKS refresh failed", "provider", name, "url", jwksURL, "error", refreshErr)
		},
	})
	if err != nil {
		// Only reachable for a genuine misconfiguration (e.g. a malformed
		// URL, caught by jwkset before any network call) — never a
		// transient fetch failure, see the comment above.
		return nil, fmt.Errorf("identity: %s: build JWKS storage for %s: %w", name, jwksURL, err)
	}

	kf, err := keyfunc.New(keyfunc.Options{Ctx: ctx, Storage: storage})
	if err != nil {
		return nil, fmt.Errorf("identity: %s: build keyfunc: %w", name, err)
	}

	return &jwksProvider{name: name, keyfunc: kf, validIssuers: validIssuers, audience: audience}, nil
}

// Verify checks idToken's signature against this provider's published
// JWKS, then its issuer, audience, and expiry — the security-critical path
// for federated account creation/login (ADR-014). A provider constructed
// with an empty audience (real credentials not yet issued for this
// environment) can never pass verification here: no real id_token will
// ever carry an empty aud claim, so this fails closed, not open.
func (p *jwksProvider) Verify(_ context.Context, idToken string) (VerifiedIdentity, error) {
	if p.audience == "" {
		return VerifiedIdentity{}, fmt.Errorf("identity: %s: provider not configured (no audience set)", p.name)
	}

	var claims idTokenClaims
	token, err := jwtlib.ParseWithClaims(idToken, &claims, p.keyfunc.Keyfunc,
		// Rejects "none"/HS256-forged tokens before the signature is even
		// checked — same defense shared/jwt.Verifier already applies to our
		// own tokens.
		jwtlib.WithValidMethods([]string{jwtlib.SigningMethodRS256.Alg()}),
		jwtlib.WithAudience(p.audience),
		jwtlib.WithExpirationRequired(),
	)
	if err != nil {
		return VerifiedIdentity{}, fmt.Errorf("identity: %s: verify id_token: %w", p.name, err)
	}
	if !token.Valid {
		return VerifiedIdentity{}, fmt.Errorf("identity: %s: id_token invalid", p.name)
	}
	// jwtlib.WithIssuer only accepts a single issuer string, but Google
	// legitimately signs with either "accounts.google.com" or
	// "https://accounts.google.com" — validated manually against the
	// allowlist instead, after the signature/audience/expiry checks above
	// have already run.
	if !slices.Contains(p.validIssuers, claims.Issuer) {
		return VerifiedIdentity{}, fmt.Errorf("identity: %s: unexpected issuer %q", p.name, claims.Issuer)
	}
	if claims.Subject == "" {
		return VerifiedIdentity{}, fmt.Errorf("identity: %s: id_token has no subject", p.name)
	}

	return VerifiedIdentity{Subject: claims.Subject, Email: claims.Email, Name: claims.Name}, nil
}
