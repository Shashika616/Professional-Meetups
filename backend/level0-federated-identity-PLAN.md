# Backend PLAN — Base Identity (Apple/Google/Email+Password), LinkedIn as Sole Trust-Granting Step, Age Eligibility

Implements ADR-014 (final shape, 2026-08-19). Read that first — this is execution detail, not the reasoning. Scope: `services/auth` and `services/gateway` only. Follows `backend/meetup-scheduling-PLAN.md`'s established pattern of a dedicated plan file per feature slice.

## Prerequisites

1. **Apple**: Apple Developer Program membership + a Services ID configured for Sign in with Apple — Shashika to create (Action Tracker §1). Not blocking Steps 1-5; those build against test id_tokens via httptest, same pattern `linkedin/client_test.go` already uses.
2. **Google**: a Google Cloud OAuth 2.0 client ID — Shashika to create (Action Tracker §1). Same non-blocking treatment.

## Step 1 — Migration `0004_base_identity_and_age.up.sql`

```sql
CREATE TYPE identity_provider AS ENUM ('apple', 'google');
-- LinkedIn deliberately NOT in this enum/table — linkedin_sub stays on
-- `users` directly (ADR-014), whether set at direct signup or linked later,
-- so there's exactly one place to check "does this user have LinkedIn."

CREATE TABLE user_identities (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider    identity_provider NOT NULL,
    subject     TEXT NOT NULL,       -- the provider's stable 'sub' claim
    email       TEXT,                -- from the verified id_token, display-only, not an identity anchor
    linked_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (provider, subject),      -- one provider identity can't attach to two users
    UNIQUE (user_id, provider)       -- a user can't link the same provider twice
);

CREATE INDEX idx_user_identities_user_id ON user_identities (user_id);

ALTER TABLE users
    ADD COLUMN age_confirmed_over_18 BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN age_confirmed_at      TIMESTAMPTZ,
    ADD COLUMN password_hash         TEXT,  -- nullable: only set for accounts that ever used the email+password path
    ALTER COLUMN trust_level SET DEFAULT 0; -- was 1; a fresh row before computeTrustLevel runs should reflect Level 0, not overclaim Level 1
```

Write the matching `.down.sql`. New `sqlc` queries in `internal/repository/queries/`: `user_identities.sql` (`InsertIdentity`, `GetIdentityByProviderSubject`, `ListIdentitiesForUser`) and additions to `users.sql` for `SetPasswordHash`, `GetUserByPersonalEmail` (needed for the recovery-on-signup check in Step 4).

## Step 2 — `internal/identity` package: Apple/Google token verification

New package, sibling to `internal/linkedin`. Apple and Google Sign-In hand the **mobile app** a signed `id_token` (JWT) directly via their native SDKs — cryptographic verification against each provider's published JWKS, no server-to-server exchange, materially different from LinkedIn's authorization-code flow. Don't force this into `linkedin/client.go`'s shape.

```go
package identity

type VerifiedIdentity struct {
    Subject string
    Email   string
    Name    string // often "" after first login for both providers — callers must handle this, don't assume it's always populated
}

type Provider interface {
    Verify(ctx context.Context, idToken string) (VerifiedIdentity, error)
}
```

- `AppleProvider`: verify against `https://appleid.apple.com/auth/keys` (fetch + cache JWKS, respect cache-control), `iss == "https://appleid.apple.com"`, `aud` matches the configured Services ID, `exp` not expired.
- `GoogleProvider`: verify against `https://www.googleapis.com/oauth2/v3/certs`, `iss` in (`accounts.google.com`, `https://accounts.google.com`), `aud` matches the configured OAuth client ID, `exp` not expired.
- **New dependency decision, flagged not silently picked**: a maintained JWKS+JWT library (e.g. `github.com/coreos/go-oidc/v3`) rather than hand-rolled fetch/cache/verify. Check `go.mod` for overlap with `shared/jwt` first (that package signs/verifies *our own* JWTs — a different concern from verifying a third party's JWKS-based token, but confirm before assuming zero overlap).
- Same `defaultTimeout = 5 * time.Second` pattern as `linkedin/client.go`/`fcm.go` for JWKS fetches.
- Unit tests mirroring `linkedin/client_test.go`: valid, expired, wrong audience, wrong issuer, malformed, JWKS-fetch-failure — table-driven, against a local self-signed test JWKS, never real Apple/Google endpoints.

## Step 3 — `internal/service/identity_resolution.go`: the shared resolution rules (ADR-014's core design)

This is the piece the whole slice hinges on — read ADR-014's Identity Resolution section again before writing this. Three functions, each backing a specific, tested invariant:

```go
// ResolveOrCreateIdentity is THE single entry point every federated login
// (Apple, Google, LinkedIn) calls — never duplicated per-RPC-handler logic.
// Looks up (provider, subject); returns the existing user if found, creates
// a new Level-0 user if not. Identical behavior regardless of which
// screen/button the client called it from — there is no backend concept of
// "this call came from a signup screen" vs "a signin screen" for federated
// methods, only "does this identity already exist."
func (s *Service) ResolveOrCreateIdentity(ctx context.Context, provider, subject, email, name string) (user repository.User, isNewUser bool, err error)

// LinkIdentityToUser attaches a new identity to an ALREADY-authenticated
// user (Profile-initiated "Connect LinkedIn", or future "add Apple/Google
// as backup"). Hard-rejects with a specific, user-facing error if (provider,
// subject) already belongs to a DIFFERENT user — never merges two accounts.
// Write an explicit test for this rejection path; it's the one real abuse
// edge in this whole design.
func (s *Service) LinkIdentityToUser(ctx context.Context, userID, provider, subject string) error

// SignUpOrRecoverWithEmail backs the email+password path. If `email`
// already matches an existing user's VERIFIED personal_email, this is
// treated as account recovery — not a duplicate create, not a hard reject —
// because the caller already proved inbox control via the OTP step before
// this is called (see Step 4). Sets password_hash on the EXISTING user and
// returns it. Otherwise creates a new Level-0 user with personal_email
// pre-verified. Document this recovery behavior prominently in the doc
// comment — it's a deliberate design choice (ADR-014), not an oversight
// that a future reader might "fix" into a hard reject.
func (s *Service) SignUpOrRecoverWithEmail(ctx context.Context, email, hashedPassword string) (user repository.User, isNewUser bool, err error)
```

Test matrix required, not optional: resolve-existing vs. create-new for Apple/Google; link-success vs. link-reject-already-claimed; email-signup-new vs. email-signup-recovers-existing. Each of these three functions should have its own table-driven test file.

## Step 4 — Email + password signup and login

- Signup requires OTP verification of the email as part of the flow — **reuse the existing `verification_codes` mechanism** (ADR-012), same table, a new `VERIFICATION_PURPOSE_EMAIL_SIGNUP` (or reuse `VERIFICATION_PURPOSE_PERSONAL_EMAIL` if the semantics line up cleanly — check before adding a new enum value that duplicates an existing one). Do not build a second OTP subsystem.
- Password hashing: **argon2id** (`golang.org/x/crypto/argon2`) — new dependency decision, flag it, don't silently pick bcrypt vs argon2id without noting the choice.
- No `username` field — the OTP-verified email is the login identifier directly (ADR-014's explicit recommendation against adding a second unique identifier).
- New RPC pair:
  ```protobuf
  rpc StartEmailSignup(StartVerificationRequest) returns (StartVerificationResponse); // reuses existing message shapes, purpose = EMAIL_SIGNUP
  rpc CompleteEmailSignup(CompleteEmailSignupRequest) returns (SessionResponse); // calls SignUpOrRecoverWithEmail internally
  rpc LoginWithPassword(LoginWithPasswordRequest) returns (SessionResponse);
  ```
  ```protobuf
  message CompleteEmailSignupRequest {
    string email = 1;
    string code = 2;            // the OTP just verified
    string password = 3;        // hashed server-side before storage, never logged
    bool age_confirmed_over_18 = 4;
  }
  message LoginWithPasswordRequest {
    string email = 1;
    string password = 2;
  }
  ```
- **Rate limiting**: extend the existing Redis-backed fixed-window limiter (currently keyed `ratelimit:{IP}:{path}`, gateway-level) to also key on the submitted email/identifier for `LoginWithPassword` and `CompleteEmailSignup` specifically — prevents credential-stuffing against a single account even from a rotating set of IPs. This is an extension of existing infrastructure, not a new subsystem — say so in the PR description.
- Password reset: reuses the same OTP mechanism again (start → verify → `SetPasswordHash`). No new subsystem.

## Step 5 — `computeTrustLevel` gets a real Level 0 branch

`internal/service/trustlevel.go` currently hardcodes the opposite assumption in its own comment ("linkedin_sub is always set by this point — Level 0/no-account isn't reachable here"). Fix:

```go
func computeTrustLevel(u repository.User) int {
    if u.LinkedInSub == "" {
        return 0
    }
    level2 := u.PhoneNumber != "" && u.PersonalEmail != "" &&
        u.LegalName != "" && u.Address != ""
    switch {
    case level2 && u.WorkEmailVerified:
        return 3
    case level2:
        return 2
    default:
        return 1
    }
}
```

Minimal diff from the original — just the new `if u.LinkedInSub == ""` branch replacing the old hardcoded-1 default. Update `trustlevel_test.go`: new cases for a federated/email-password-only user (0), and the 0→1 transition when LinkedIn links later. Confirm existing 1/2/3 cases still pass unchanged.

**Required audit, not optional**: grep every place that currently treats "has a valid JWT" as synonymous with "trust_level >= 1" — an authenticated Level-0 caller is now a real, expected case. Check `services/meetup`'s `trustgate.go` too — its floor checks should already correctly reject 0 since the floor is ≥2, but verify rather than assume.

## Step 6 — RPCs and gateway routes

```protobuf
enum IdentityProviderProto {
  IDENTITY_PROVIDER_UNSPECIFIED = 0;
  IDENTITY_PROVIDER_APPLE = 1;
  IDENTITY_PROVIDER_GOOGLE = 2;
}

// Unauthenticated — calls ResolveOrCreateIdentity. Rejects with a clear,
// specific error if age_confirmed_over_18 is false and this is a new user —
// server enforces this regardless of client UI state.
rpc CompleteFederatedSignup(CompleteFederatedSignupRequest) returns (SessionResponse);

// Authenticated — gateway sets user_id from the verified JWT. Calls
// LinkIdentityToUser; hard-rejects on collision (Step 3).
rpc LinkIdentity(LinkIdentityRequest) returns (SessionResponse);
```

```protobuf
message CompleteFederatedSignupRequest {
  IdentityProviderProto provider = 1;
  string id_token = 2;
  bool age_confirmed_over_18 = 3;
}
message LinkIdentityRequest {
  string user_id = 1; // set by gateway from verified JWT
  IdentityProviderProto provider = 2;
  string id_token = 3;
}
```

**`CompleteLinkedInOnboarding` (existing RPC) is unchanged in its account-creation behavior** — still unauthenticated, still resolve-or-creates via LinkedIn directly, still grants Level 1 immediately. The one addition: it needs `age_confirmed_over_18` added to `CompleteLinkedInOnboardingRequest` and the same server-side rejection-if-false as the other signup paths, since LinkedIn direct signup didn't have an age gate before and now needs one too (ADR-014).

Gateway (`services/gateway/internal/handlers/handlers.go`):

```go
mux.HandleFunc("POST /v1/auth/federated/signup", h.federatedSignup)       // unauthenticated, new
mux.HandleFunc("POST /v1/auth/email/signup", h.emailSignup)               // unauthenticated, new
mux.HandleFunc("POST /v1/auth/email/login", h.emailLogin)                 // unauthenticated, new
mux.HandleFunc("POST /v1/auth/linkedin/callback", h.linkedInCallback)     // UNCHANGED — line 43 today, stays unauthenticated
mux.Handle("POST /v1/auth/identities/link", h.requireAuth(http.HandlerFunc(h.linkIdentity))) // authenticated, new — Profile-initiated linking only
```

No breaking change to any existing route in this version of the plan — everything new is additive.

## Step 7 — Self-review checklist

- [ ] `age_confirmed_over_18 = false` rejected server-side on every signup path (Apple, Google, email, **and LinkedIn** — don't miss LinkedIn, it didn't have this check before).
- [ ] Apple/Google id_token verification actually validates signature + issuer + audience + expiry.
- [ ] `LinkIdentityToUser` rejects a subject already linked to a different user — tested explicitly.
- [ ] `SignUpOrRecoverWithEmail`'s recovery path only triggers on an already-*verified* `personal_email` match, never an unverified one — tested explicitly, since this is the one place cross-account resolution happens automatically and getting the condition wrong would be a real account-takeover bug.
- [ ] No provider id_token, LinkedIn access token, raw password, or JWKS private material ever logged.
- [ ] Passwords hashed with argon2id before storage, never compared in non-constant-time, never included in any response.
- [ ] `computeTrustLevel`'s Level-0 branch has test coverage; existing 1/2/3 tests still pass unchanged.
- [ ] Grep audit (Step 5) actually performed and documented.
- [ ] Login-attempt rate limiting keyed on identifier, not just IP, verified with a test.

## Related

ADR-014 · `frontend/level0-federated-identity-PLAN.md` · `backend/PLAN.md` (original LinkedIn slice, unchanged by this plan) · `backend/meetup-scheduling-PLAN.md` (pattern this follows)
