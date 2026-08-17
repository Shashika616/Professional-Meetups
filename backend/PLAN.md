# Onboarding Slice — Execution Plan

Give this whole file to Claude Code as the task brief. It's written to be
followed literally, in order. Scaffolding (docker-compose, Dockerfiles, SQL
migrations, the .proto contract, go.work/go.mod files) already exists —
this plan is for the Go source code, which doesn't exist yet.

**Correction (2026-08-17), after this shipped and was tested against real
LinkedIn accounts:** PKCE (RFC 7636), specified throughout `internal/linkedin/`
below, does not work against LinkedIn's actual Sign In with LinkedIn / OpenID
Connect product — including `code_challenge`/`code_verifier` makes LinkedIn
reject the token exchange with `401 invalid_client` ("Client authentication
failed"), even with fully correct credentials. Confirmed by hand-building
the entire OAuth round trip outside this app (a real browser login, then a
direct `curl` to `https://www.linkedin.com/oauth/v2/accessToken`): identical
requests succeed with PKCE omitted and fail with it included, for the same
client_id/client_secret/redirect_uri. LinkedIn's own `/oauth/v2/accessToken`
docs don't list `code_verifier` as a supported parameter for this product
either. **PKCE has been removed** from both this backend and the frontend —
`pkce_verifier` is gone from the proto (reserved, not reused), `ExchangeCode`
takes no verifier. Security is still sound without it: this is a
confidential client (the `client_secret` lives only in this backend, never
in the mobile app), which is the property PKCE exists to substitute for on
a public client that can't hold a secret. The PKCE flow described below is
kept for historical context (why the code originally looked this way) —
don't rebuild it.

**Scope boundary — read this first:** this slice is LinkedIn federated
(Level 1a) onboarding *only*. Do not build phone/personal-email/personal-
details verification (Level 2), corporate email (Level 3), KYC (Level 4),
matching, messaging, or the Level 1b "pasted URL" fallback. Those are later
slices with their own migrations and services. If a design decision here
seems to imply one of those, stop and flag it rather than building toward
it — see `docs/00-project/cowork-operating-charter.md` for why.

## Step 0 — Prerequisites (human, not Claude Code)

Before any code gets written, Shashika needs to:
1. Install Go: `brew install go`, confirm with `go version`.
2. Register a LinkedIn OAuth app at linkedin.com/developers/apps with the
   "Sign In with LinkedIn using OpenID Connect" product, and note the
   client ID/secret.
3. Follow `backend/README.md` "First-time setup" to generate the local JWT
   RSA keypair and fill in `.env`.

Claude Code should confirm these are done (check `go version` succeeds,
check `backend/.env` and `backend/secrets/` exist) before proceeding — don't
silently work around their absence.

## Step 1 — Pin the real Go version

`go.work` and every `go.mod` currently say `go 1.23` as a placeholder. Run
`go version`, and if it's newer, bump all four files (`go.work`,
`shared/go.mod`, `services/gateway/go.mod`, `services/auth/go.mod`) to match
the actual installed version. Don't leave a stale version directive.

## Step 2 — Generate the gRPC code from the .proto

Use `buf` (simpler than raw `protoc`): `brew install bufbuild/buf/buf`, then
generate Go code from `backend/proto/auth/v1/auth.proto` into
`backend/shared/proto/auth/v1/` (matching the `go_package` option already in
the .proto file). Commit the generated `.pb.go` files — this repo commits
generated code so a fresh clone builds without requiring `buf` at all,
consistent with `.gitignore`'s comment about this.

## Step 3 — `shared/` package

Nothing in `shared/` should import from `services/gateway` or
`services/auth` — it's a one-way dependency, enforced by convention (Go
doesn't stop you, but a reviewer should).

- **`shared/jwt/`** — RS256 sign/verify using `github.com/golang-jwt/jwt/v5`.
  - `Claims` struct: `UserID string`, `TrustLevel int`, standard registered
    claims (`exp`, `iat`, `iss`, `sub`).
  - `Signer` — loads the private key from a file path (constructor takes the
    path, fails fast at startup if the key won't parse — never fail lazily
    on the first request). `Sign(claims Claims) (string, error)`.
  - `Verifier` — loads only the public key. `Verify(token string) (Claims, error)`.
    The gateway only ever constructs a `Verifier`; only the auth service
    constructs a `Signer` — enforce this by which service imports which
    constructor, matching ADR-009's "only the auth service holds the
    private key."
  - Access tokens: 15 minute expiry, hardcoded constant, not configurable
    via env (a security parameter, not a deployment parameter).

- **`shared/proto/auth/v1/`** — generated code from Step 2.

- **`shared/logging/`** — thin wrapper around `log/slog` with JSON output
  (matches Cloud Logging's expected format). A middleware/interceptor
  (one for HTTP, one for gRPC) that generates or propagates a
  `X-Request-ID`/correlation ID and attaches it to every log line for that
  request — this is the seed of the "logging that scales across services"
  requirement from earlier discussion; full observability design is
  explicitly out of scope for this slice, this is just making sure the
  hooks exist so it isn't bolted on later.

- **`shared/apperror/`** — a small set of sentinel errors
  (`ErrNotFound`, `ErrInvalidInput`, `ErrUnauthorized`, `ErrConflict`,
  `ErrInternal`) that both services use via `errors.Is`/`fmt.Errorf("...: %w", ...)`
  wrapping, plus one function mapping each to the right gRPC status code and
  one mapping gRPC status codes to HTTP status codes (used at the gateway
  boundary). One mapping table, not five different ad-hoc switch statements
  scattered through handlers.

## Step 4 — `services/auth`

```
services/auth/
├── cmd/server/main.go       # wiring only: load config, construct dependencies, start gRPC server, handle SIGTERM gracefully
└── internal/
    ├── config/               # env var loading, fails fast on missing required vars
    ├── linkedin/             # OIDC client
    ├── repository/           # Postgres access
    ├── service/              # business logic, implements the generated AuthServiceServer interface
    └── events/               # Pub/Sub publisher
```

### `internal/linkedin/` — the OIDC client

LinkedIn uses standard OAuth 2.0 + OpenID Connect with **PKCE** (RFC 7636).
PKCE matters here specifically because the *app* (not just this backend) is
part of the flow, and mobile apps can't safely hold a client secret — PKCE
protects the authorization code in transit between LinkedIn's redirect and
this backend's exchange call, even though this backend still separately
uses `LINKEDIN_CLIENT_SECRET` for its own confidential-client exchange with
LinkedIn. Two different protections, don't conflate them.

Flow (full sequence, for reference — implement the backend's half):
1. **App** generates `code_verifier` (random 43-128 char string) and
   `state`, computes `code_challenge = BASE64URL(SHA256(code_verifier))`.
2. **App** opens LinkedIn's authorization URL directly (no backend round
   trip needed to start — `client_id` is public) with `code_challenge`,
   `code_challenge_method=S256`, `state`, `redirect_uri`, `scope=openid profile email`,
   `response_type=code`.
3. LinkedIn redirects to `redirect_uri`. **Correction, added after this step
   shipped (see `frontend/PLAN.md`):** LinkedIn requires an absolute
   `https://` redirect URL — it does not accept a custom scheme directly.
   `redirect_uri` is therefore an HTTPS bridge page that immediately
   forwards to the app's custom scheme (e.g.
   `professionalconnections://auth/linkedin/callback`) with `code` and
   `state` preserved. The backend code below is unaffected either way —
   `redirect_uri` is just an opaque string it echoes back to LinkedIn's
   token endpoint, matching whatever the app sent at step 2.
4. **App** verifies `state` matches what it generated, then calls this
   backend's `POST /v1/auth/linkedin/callback` with `authorization_code`,
   `code_verifier`, `redirect_uri`.
5. **Backend** (this package) exchanges `code` + `code_verifier` +
   `client_secret` with LinkedIn's token endpoint for a LinkedIn access
   token, then calls LinkedIn's OIDC userinfo endpoint to get `sub`, `name`,
   `picture`.
6. **Backend discards the LinkedIn access token immediately after reading
   userinfo — it is never persisted.** Same minimal-retention principle as
   ADR-003's work-email handling: extract what's needed, discard the rest.

Functions needed: `ExchangeCode(ctx, code, verifier, redirectURI) (linkedinToken, error)`,
`FetchUserInfo(ctx, linkedinToken) (UserInfo, error)`. Use `net/http` directly
with a configured `http.Client` (explicit timeout, e.g. 5s — never use the
zero-value default client, it has no timeout). Table-driven tests against an
`httptest.Server` standing in for LinkedIn's endpoints — do not hit real
LinkedIn in tests.

### `internal/repository/` — repository pattern

Interfaces first, Postgres implementation second — mirrors the pattern
already established on the Flutter side (`AuthService`/`MatchingService`
abstract interfaces + `Mock*` implementations, see `docs/00-project/...`
CLAUDE.md's "Service-contract pattern"). Same idea here: define
`UserRepository` and `RefreshTokenRepository` interfaces in
`internal/repository/`, so `internal/service/` depends on the interface, not
directly on Postgres — makes the business logic testable without a real DB
if that's ever wanted, and is the idiomatic Go way to draw this boundary
(no DI framework/magic, just passing a constructor an interface value).

Use `github.com/jackc/pgx/v5` with `pgxpool.Pool` for connection pooling —
not `database/sql` directly, `pgx` is faster and has better Postgres-type
support. Use `sqlc` (`https://sqlc.dev`) to generate the query layer from
plain SQL files instead of hand-writing query code or reaching for an ORM —
every generated query is parameterized by construction, which is how SQL
injection gets eliminated structurally rather than by discipline (see
`docs/00-project/cowork-operating-charter.md`-adjacent reasoning from the
earlier code-standards discussion). Write the `.sql` query files under
`internal/repository/queries/`, configure `sqlc.yaml` at `services/auth/`
root.

`RefreshTokenRepository` needs: `Create(ctx, userID, tokenHash, expiresAt) (id, error)`,
`FindByHash(ctx, tokenHash) (*RefreshToken, error)`,
`Rotate(ctx, oldID, newTokenHash, newExpiresAt) (newID, error)` (single
transaction: mark old row's `replaced_by`, insert new row — don't do this as
two separate non-transactional calls), `Revoke(ctx, tokenHash) error`.
Never store or log a raw refresh token — only ever the SHA-256 hash
(`crypto/sha256`, hex-encoded). This is why `token_hash` is what's indexed
in the migration, not the raw token.

### `internal/service/`

Implements the generated `AuthServiceServer` gRPC interface. Constructor
takes the repository interfaces, the LinkedIn client, the JWT signer, and
the event publisher — all as explicit parameters (dependency injection via
constructor, no framework). This is where `CompleteLinkedInOnboarding`
lives: call LinkedIn client → look up existing user by `linkedin_sub` (or
create one, in a transaction) → issue access token (JWT signer) → create
refresh token row → publish `user.onboarded` if this was a new user → return
`SessionResponse`. `RefreshSession` and `RevokeSession` similarly thin,
delegating to the repository + signer.

### `internal/events/`

One function: `PublishUserOnboarded(ctx, userID, trustLevel) error`. Event
payload is `{user_id, trust_level, occurred_at}` — no name, no photo, no PII
beyond the ID, matching the data-minimization principle applied everywhere
else in this project. Topic name: `user-onboarded` (Pub/Sub topics use
kebab-case by convention).

### Tests

- Unit: `internal/linkedin` (against `httptest`), `internal/repository`
  query-building where `sqlc` doesn't already guarantee correctness,
  `shared/jwt` sign/verify round-trip including expiry and tampered-token
  rejection.
- Integration: one test that spins up against the real `docker-compose`
  Postgres (or a GitHub Actions Postgres service container in CI — see Step
  6), runs the actual migrations, and exercises
  `CompleteLinkedInOnboarding` end to end against a stubbed LinkedIn
  (`httptest`), asserting a user row and a refresh-token row actually exist
  afterward. This is the test that would have caught a bad migration or a
  bad SQL query that unit tests with mocks wouldn't.

## Step 5 — `services/gateway`

```
services/gateway/
└── internal/
    ├── config/
    ├── handlers/        # REST endpoints
    ├── middleware/       # rate limiting, request logging, panic recovery
    └── authclient/       # gRPC client wrapper around the auth service
```

Use the standard library's `net/http` with the Go 1.22+ enhanced
`ServeMux` (method + path-parameter routing is now built in — no need for
a third-party router for an API this size).

### REST contract (what the Flutter app calls)

```
POST /v1/auth/linkedin/callback
  body:  {"authorization_code": "...", "redirect_uri": "..."}  # no code_verifier — see the 2026-08-17 correction at the top of this file
  200:   {"user_id": "...", "access_token": "...", "refresh_token": "...",
          "expires_in": 900, "is_new_user": true}
  400:   invalid/expired code
  429:   rate limited

POST /v1/auth/refresh
  body:  {"refresh_token": "..."}
  200:   same shape as above (new pair)
  401:   token invalid, expired, or already-rotated (possible theft — treat as revoked)

POST /v1/auth/logout
  body:  {"refresh_token": "..."}
  200:   {"success": true}   # idempotent, always 200 even if the token was already invalid
```

Refresh tokens are returned in the JSON body here because this slice is
mobile-only (ADR-007) and mobile clients store them in OS secure storage
directly (ADR-009) — not cookies. **When the web admin dashboard gets
built later, the gateway will need platform-aware behavior (e.g. an
`X-Client-Platform` header) to switch to httpOnly-cookie mode for that
client instead.** Don't build that branch now for a client that doesn't
exist yet — just don't design these handlers in a way that makes adding it
later awkward (keep token-issuance logic in one place, not scattered).

### `internal/middleware/` — rate limiting

Redis-backed fixed-window counter, applied to all three `/v1/auth/*`
routes specifically (pre-auth endpoints are the most abuse-prone surface —
brute-forcing `/refresh`, hammering `/linkedin/callback`). Key:
`ratelimit:{ip}:{route}`, `INCR` + `EXPIRE 60` on first increment, reject
with 429 above 20 requests/minute. This is a real, working rate limiter,
not a placeholder — but it's intentionally the simplest correct algorithm
(fixed window), not a token bucket or sliding window; upgrade later only if
the fixed-window edge-effect (bursts at window boundaries) actually becomes
a problem.

### `internal/authclient/`

Thin wrapper constructing a gRPC client connection to `AUTH_SERVICE_ADDR`
at startup (fail fast if it can't connect) and exposing typed Go methods
that call the generated gRPC client underneath — handlers call this, never
the generated gRPC stub directly, so the gRPC-specific error handling lives
in one place.

### Tests

Table-driven handler tests using `httptest.NewRecorder`, with a fake
`authclient` implementation (interface, not the real gRPC one) so these
tests don't need a running auth service. One rate-limiter test against a
real local Redis (docker-compose already provides one).

## Step 6 — CI

Add a `backend` job to `.github/workflows/` (new file, e.g.
`backend-ci.yml`, alongside the existing `flutter-ci.yml` — don't merge
them into one file, they're unrelated toolchains). Mirror the Flutter CI's
own recently-learned lesson: **pin the exact Go version explicitly**
(`actions/setup-go@v5` with `go-version: 'x.y.z'` matching Step 1's
number), not `stable` — an unpinned toolchain silently drifting is exactly
the class of bug already fixed once in this repo's Flutter CI, don't
reintroduce it here.

Job steps: checkout → setup-go → `go build ./...` across the workspace →
`go vet ./...` → `golangci-lint run` (add a `.golangci.yml` with at least
`errcheck`, `staticcheck`, `govet`, `ineffassign` enabled) → `go test ./...`
with a Postgres service container (`postgres:16-alpine`, matching the
compose file) for the integration test in Step 4.

## Step 7 — Self-review checklist (do this before calling it done)

- [ ] No raw LinkedIn access/refresh token ever written to a log line, a DB
      column, or an error message.
- [ ] No raw refresh token (ours) ever logged — only its hash.
- [ ] Every SQL query is parameterized (guaranteed by `sqlc` if Step 4 was
      followed — spot-check anyway).
- [ ] JWT `Signer` construction happens only in `services/auth`; grep the
      gateway module to confirm it never imports anything that could
      construct one.
- [ ] `/v1/auth/*` rate limiting is actually wired into the router, not
      just implemented and unused.
- [ ] Every exported function that can fail returns `error`, not a panic —
      panics in request-handling paths become 500s via a recovery
      middleware, not crashes.
- [ ] `go vet` and `golangci-lint` both clean.
- [ ] `docker compose up --build` works from a totally clean checkout
      (`docker compose down -v` first) — this is the real end-to-end proof,
      not just "the Go code compiles."
- [ ] Bring the diff back to Cowork for an architecture/security pass
      before merging, the same way the Flutter frontend got reviewed
      earlier in this project.

## Explicitly not in this slice

Level 1b (pasted-URL LinkedIn), phone/personal-email/personal-details
(Level 2) and corporate email (Level 3) — **superseded by the addendum
below, now in scope** — KYC (Level 4), the matching engine, messaging, the
Realtime Gateway, and the API Gateway's Cloud Run/Load Balancer deployment
manifests (Cloud Run YAML / Terraform — this plan only covers the code and
local Docker setup). Flag rather than build if asked to extend into any of
these without an explicit go-ahead.

---

# Addendum (2026-08-17): Level 2/3 Verification Slice

Everything above this line is the original Level 1a (LinkedIn) plan,
executed, reviewed, committed. This addendum is the next bounded slice —
Level 2 (phone, personal email, personal details) and Level 3 (corporate
email), per [[ADR-012]] (already Accepted — the vendor/scope decisions
below aren't up for re-litigation, only the execution detail is new here).
Give Claude Code this whole file; the addendum assumes everything above it
already exists and works.

**Sequencing note**: hand this to Claude Code only after the currently
in-flight `frontend/PLAN.md` Steps 12-15 diff (session UX / homepage
personalization) has been reviewed and merged — both touch overlapping
frontend files (`app_providers.dart`, `ProfilePage`), and running two
Claude Code sessions against the same files concurrently risks a messy
merge neither session can see coming.

## Scope boundary for this addendum

Build: phone OTP verification, personal email OTP verification, personal
details (legal name + address) capture, corporate email OTP verification
(MVP-scoped per ADR-012 — no domain-reputation checks, no company
verification database, no manual review), a real `GET /v1/users/me`, and
server-side `trust_level` recomputation whenever any of the above
completes. **Phone, personal email, and corporate email all share one
backend-owned OTP mechanism** (ADR-012's 2026-08-17 correction — Firebase
Phone Auth was reconsidered and dropped in favor of this, see the ADR for
why) — generate a code, hash it, store it with an expiry, send it via a
pluggable sender interface, verify it the same way for all three.

**Do not build**: the full Verification Model § 5 corporate-email
fraud-detection flow (domain age/SPF/DKIM/DMARC, company verification
database, manual review for unknown domains — all explicitly deferred by
ADR-012). **Named risk, accepted for this MVP, not a bug to fix here**: a
fresh lookalike domain (`acme-hr.com` for real company `acme.com`) passes
this MVP's corporate-email check cleanly — `work_email_verified` proves
OTP-confirmed mailbox control on that domain, nothing about whether the
domain is a real employer. Don't let any field/response naming imply
otherwise (`work_email_verified` is accurately named; don't add a
differently-named field elsewhere that overstates it, e.g. anything like
`employer_verified`). KYC (Level 4), the 90-day work-email re-verification *job*
(store `work_email_verified_at` so a future job can use it — don't build
the job itself), phone/email-as-a-login-method (this is verification for
trust level, not an alternate way to authenticate — LinkedIn stays the
only sign-in path), re-linking a different LinkedIn account to an
existing phone/email-anchored account (ADR-012 flags this as still open).

## Step A — Human prerequisite: transactional email + SMS providers

**Both providers are now decided (2026-08-17).** SMS: Twilio, specifically
the plain Programmable Messaging API, not Twilio Verify. Twilio Verify is a
fully-managed OTP product (generates/sends/checks the code entirely on
Twilio's side) — deliberately not used, because it would make phone a
second, separate OTP mechanism again, undoing the point of this
addendum's unification. Plain Messaging API is just an SMS carrier — your
own backend keeps owning code generation/storage/verification for phone
exactly like it already does for email.

`backend/.env.example` now has empty `TWILIO_ACCOUNT_SID`,
`TWILIO_AUTH_TOKEN`, `TWILIO_PHONE_NUMBER` placeholders — Shashika fills
these in from the Twilio Console (Account SID + Auth Token are on the
console dashboard; the phone number is a Twilio number provisioned for
sending, E.164 format). Build the real `TwilioSmsSender` (Step C/D) behind
the `SmsSender` interface now — it's decided, not deferred like email —
but `LoggingSmsSender` should still exist as the implementation tests and
local dev use by default (switch via config, same pattern as picking
between `flutter run` targets), so nothing breaks if the env vars are
still empty when Claude Code runs this.

Email: Resend. `backend/.env.example` now has empty `RESEND_API_KEY` and
`RESEND_FROM_EMAIL` placeholders — Shashika fills in the API key from the
Resend dashboard, and the from-address once a domain is verified with
Resend (DNS records Resend provides). **Until a domain is verified,
`RESEND_FROM_EMAIL` can be set to Resend's sandbox address
(`onboarding@resend.dev`), which works without domain verification for
testing** — mention this as an option, don't require a fully verified
domain to be functional. Same fallback pattern as SMS: build a real
`ResendEmailSender` behind `EmailSender` now, but fall back to
`LoggingEmailSender` when `RESEND_API_KEY` is empty, so nothing breaks
before Shashika fills it in.

## Step B — Migration

New file, `db/migrations/0002_level_2_3_verification.up.sql` (+ matching
`.down.sql`). Add to `users`, mirroring the existing `linkedin_sub`
pattern of "presence of the value IS the verified signal," no separate
boolean where avoidable:

- `phone_number TEXT` — nullable, `UNIQUE` via a partial index
  (`WHERE phone_number IS NOT NULL`), same shape as `idx_users_linkedin_sub`.
- `personal_email TEXT` — nullable, same partial-unique-index treatment.
- `legal_name TEXT`, `address TEXT` — nullable, self-reported, no
  verification (Verification Model § 4). A single free-text `address`
  field is enough for this MVP — don't over-structure it into
  line1/city/postal columns for a feature that doesn't need to query on
  those parts yet.
- `company_domain TEXT` — nullable, extracted from a verified corporate
  email, never the raw address (ADR-003, unchanged).
- `work_email_verified BOOLEAN NOT NULL DEFAULT false` — named exactly per
  ADR-012, kept as an explicit boolean (unlike the others above) because
  ADR-012's own § Decision names it as its own field.
- `work_email_verified_at TIMESTAMPTZ` — nullable, feeds the future
  90-day-re-verify job (not built here).

New table `verification_codes` — reused across all three OTP-based
verifications (phone, personal email, corporate email):

```sql
CREATE TYPE verification_purpose AS ENUM ('phone', 'personal_email', 'corporate_email');

CREATE TABLE verification_codes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    purpose     verification_purpose NOT NULL,
    target      TEXT NOT NULL,       -- the phone number or email address being
                                      -- verified; deleted once verified (ADR-003
                                      -- for the corporate-email case) or expired
    code_hash   TEXT NOT NULL,       -- SHA-256, same pattern as refresh_tokens.token_hash
    attempts    SMALLINT NOT NULL DEFAULT 0,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, purpose)  -- one pending code per user per purpose; a
                                 -- new send overwrites (upsert), it doesn't stack
);
```

`UNIQUE (user_id, purpose)` is what makes the 1-minute resend timer
server-enforceable (Step G) — you can look up "is there already a
non-expired code for this user+purpose, and how old is it" with one
indexed lookup.

## Step C/D — `EmailSender` + `SmsSender` interfaces, shared OTP generation/verification

New `internal/email/` package:

```go
type EmailSender interface {
    SendVerificationCode(ctx context.Context, to, code string, purpose Purpose) error
}
```

New `internal/sms/` package, same shape:

```go
type SmsSender interface {
    SendVerificationCode(ctx context.Context, to, code string) error
}
```

**Both `EmailSender` and `SmsSender` get two implementations each**,
same pattern for both: a `Logging*` version (logs `to`/`code`/`purpose`
via `slog` at info level instead of sending — used by default and by
every automated test) and a real provider-backed version — `ResendEmailSender`
(Resend's Go SDK, `resend-go`, using `RESEND_API_KEY`/`RESEND_FROM_EMAIL`
from Step A) and `TwilioSmsSender` (plain Messaging API, using
`TWILIO_ACCOUNT_SID`/`TWILIO_AUTH_TOKEN`/`TWILIO_PHONE_NUMBER`). Which
implementation is actually wired up in `main.go` should be a config
switch — use the real sender only if its required env vars are all
non-empty, fall back to the `Logging*` version otherwise — so local
dev/tests keep working even before Shashika fills in real credentials for
either or both.

**OTP generation/verification is one shared code path used by all three
purposes** (phone, personal email, corporate email) — don't write it
three times. A cryptographically random 6-digit code (`crypto/rand`, not
`math/rand`), hashed with SHA-256 before storage (mirroring
`refresh_tokens`), 10-minute expiry, `attempts` capped at 5 before the
code is invalidated and a fresh send is required (prevents brute-forcing
a 6-digit space — 5 attempts against a 10-minute-lived code is a
reasonable bound; document the reasoning in a comment same as the
existing rate-limiter's comments do). The only thing that differs per
purpose is which `Sender` interface dispatches the message (`SmsSender`
for `'phone'`, `EmailSender` for the other two) and, for
`'corporate_email'` specifically, the free/role-based domain rejection
check (Verification Model § 5) that runs *before* a code is generated at
all.

## Step E — Proto: new `AuthService` RPCs

Extend `proto/auth/v1/auth.proto` (regenerate via `buf`, same as before).
All of these require an authenticated caller — the gateway extracts
`user_id` from the verified JWT and passes it explicitly; never trust a
client-supplied user ID (same principle as everything else in this slice):

```protobuf
rpc StartPhoneVerification(StartVerificationRequest) returns (StartVerificationResponse);
rpc VerifyPhoneCode(VerifyCodeRequest) returns (SessionResponse);
rpc StartPersonalEmailVerification(StartVerificationRequest) returns (StartVerificationResponse);
rpc VerifyPersonalEmailCode(VerifyCodeRequest) returns (SessionResponse);
rpc SubmitPersonalDetails(SubmitPersonalDetailsRequest) returns (SessionResponse);
rpc StartCorporateEmailVerification(StartVerificationRequest) returns (StartVerificationResponse);
rpc VerifyCorporateEmailCode(VerifyCodeRequest) returns (SessionResponse);
rpc GetProfile(GetProfileRequest) returns (ProfileResponse);
```

`StartVerificationRequest`/`VerifyCodeRequest` are shared, generic
messages across all three purposes (a `purpose` enum field plus
`target`/`code` as needed) — phone is no longer a special case requiring
its own request/response shapes, which is the actual point of the
ADR-012 correction: one mechanism, three purposes, not three mechanisms.

Design choices worth calling out explicitly:

- **Verification-completing RPCs return a fresh `SessionResponse`** (new
  access token, not just a bare success ack). `trust_level` lives only in
  the JWT (Step 3 of the original plan) — if verifying your phone didn't
  reissue a token, the app would show your old trust level for up to 15
  minutes until the next natural refresh, which is a confusing UX for an
  action that just happened. Reissue immediately instead.
- **`StartVerificationResponse` is intentionally near-empty** (maybe
  just `int32 resend_after_seconds`) — never echo the code or any
  indication of whether the phone/address already exists on another
  account (that's a user-enumeration leak; see Step F's error-mapping
  note).
- **`GetProfile`** returns everything `ProfilePage` needs to render real
  state: `phone_verified` (derived from `phone_number IS NOT NULL`,
  never the number itself over the wire — same "never reveal full phone
  number" rule as Verification Model § 1), `personal_email_verified`,
  `legal_name`/`address` presence (booleans, not values — profile edit
  screens are out of scope, so there's no need to round-trip the actual
  address text yet), `company_domain` + `work_email_verified`,
  `trust_level`. This is the first real use of a dedicated profile
  endpoint — the LinkedIn callback's inline fields (Step 3, original
  plan) stay as they are for session establishment.

## Step F — Gateway: REST endpoints + auth middleware

New endpoints, all under `/v1/verification/*` except `GET /v1/users/me`:
`POST /v1/verification/phone/start`, `POST /v1/verification/phone/verify`,
`POST /v1/verification/personal-email/start`,
`POST /v1/verification/personal-email/verify`, `POST /v1/verification/personal-details`,
`POST /v1/verification/corporate-email/start`, `POST /v1/verification/corporate-email/verify`,
`GET /v1/users/me`.

**All of these need a new auth middleware** — everything before this
addendum was either unauthenticated (LinkedIn callback) or worked from a
refresh token in the request body. These work from the `Authorization:
Bearer <access_token>` header instead: verify the JWT, extract `user_id`,
reject with 401 if missing/invalid/expired before the handler runs.

**Confirmed by reading the actual gateway code (not assumed): the gateway
does not construct or use a `shared/jwt.Verifier` at all yet.**
`JWT_PUBLIC_KEY_PATH` is already set in `docker-compose.yml` and even
mentioned in a comment in `services/gateway/internal/config/config.go`,
but that env var isn't actually read into the `Config` struct or used
anywhere in gateway code today — the gateway has had no reason to verify
a token itself until this addendum. This is genuinely new wiring, not a
reuse of something that already runs: add `JWTPublicKeyPath` to gateway's
`Config`, construct a `jwt.Verifier` in `cmd/server/main.go` (same
`jwt.NewVerifier(publicKeyPath)` constructor the auth service's own
`Verifier` usage already establishes the shape of), and build the new
middleware around it. Add this as a new step in the middleware chain,
applied only to the `/v1/verification/*` and `/v1/users/me` routes, not
globally — the LinkedIn/refresh/logout routes stay unauthenticated-at-the-
gateway-layer, as they are today.

**Error mapping**: a `StartXVerification` call for a phone/email already
verified on a *different* account should not say "already in use" (user
enumeration) — map to the same generic error a real send failure would
give. A resend attempted before `resend_after_seconds` has elapsed maps to
a distinct, specific error the client can show as "please wait" rather
than a generic failure (Step G). `StartCorporateEmailVerification` for a
free/role-based-rejected domain (Step C/D's pre-check) maps to its own
distinct error — this one *is* safe to be specific about ("please use
your work email, not a personal address") since it reveals nothing about
account existence, only about the domain the user themselves just typed.

**Race condition worth handling explicitly**: two different users can
each hold a pending code for the same phone number/email simultaneously
(`verification_codes` has no uniqueness on `target`, only on `(user_id,
purpose)`) — whichever completes `VerifyXCode` first writes it to their
`users` row, protected by that column's `UNIQUE` constraint (Step B). The
second user's otherwise-correct code then hits a `UNIQUE` violation on
write. Map that to `apperror.ErrConflict` with a clear "this
number/address is already verified on a different account" message —
safe to be specific here too, since by this point the user has already
proven control of the target via a correct OTP, so there's no fresh
enumeration leak. Same pattern the existing concurrent-registration race
in `CompleteLinkedInOnboarding` already established — don't leave this
one unhandled the way that one was noted as a non-blocking gap.

## Step G — Rate limiting / server-enforced resend timer

Add the new `/v1/verification/*` routes to the existing Redis-backed
fixed-window limiter (`internal/middleware/ratelimit.go` — confirmed
generic, keyed by `(IP, route)`, so this is registering the new routes
with it in `cmd/server/main.go`, not writing a new limiter). Same 20/min
default is fine — **don't also build a separate per-user limit**: the
`verification_codes` cooldown check below already stops one user from
hammering one target regardless of which IP they call from, which is the
actual gap IP-based limiting alone has. Two overlapping defenses for the
same threat is unnecessary complexity here.

Separately, and just as importantly: **the 1-minute resend cooldown is
enforced server-side**, via `verification_codes`' `UNIQUE (user_id,
purpose)` row — `StartXVerification` checks the existing row's
`created_at` before overwriting it, and rejects with the "please wait"
error (Step F) if under 60 seconds old. The client's countdown timer
(frontend addendum) is a UX convenience, not the actual control — never
trust a client-reported "the timer expired" claim.

## Step H — Trust level recomputation

New small function, `internal/service/trustlevel.go` (or similar) —
`computeTrustLevel(user) int`, pure function, called after every mutation
in Steps C/D-F that changes a verification field, before the row is
persisted or a fresh `SessionResponse`/JWT is issued:

```go
func computeTrustLevel(u User) int {
    level2 := u.PhoneNumber != "" && u.PersonalEmail != "" &&
        u.LegalName != "" && u.Address != ""
    switch {
    case level2 && u.WorkEmailVerified:
        return 3
    case level2:
        return 2
    default:
        return 1 // linkedin_sub is always set by this point — Level 0/no-account isn't reachable here
    }
}
```

**Explicit and load-bearing, not just illustrative pseudocode**: Level 3
requires Level 2's four conditions to *all* still hold, not just
`work_email_verified` in isolation — a user who verified only corporate
email while skipping phone/personal-email/personal-details must compute
to Level 1, not Level 3. This matches [[Trust Levels]]'s literal
definition ("Level 3... on top of Level 2") — get the switch statement's
ordering right (`level2 && work_email_verified` checked first) rather
than checking `work_email_verified` alone anywhere. There is deliberately
no partial credit for 1-of-4 Level 2 fields — Trust Levels defines Level
2 as the bundle, not four independent gates; the separate *continuous*
trust score (Trust & Safety Architecture § Continuous Trust Score) is
where partial-progress signals belong, and that system is explicitly not
part of this addendum. (Level 4/KYC isn't reachable yet — no code path
can produce it.) Keep this logic in exactly one function, same
"single mapping table" discipline `apperror` already uses — trust-level
rules should never be duplicated across multiple call sites.

## Step I — Update `backend/ARCHITECTURE.md`

Same discipline the original plan's Step 3 used — this file is
documented as "the one to read when something breaks," and it goes stale
the moment new files/packages/routes exist that it doesn't mention. Add:
`internal/email/`, `internal/sms/`, `internal/service/trustlevel.go`, the
new migration, the new `/v1/verification/*` + `GET /v1/users/me` routes
and what each does, and the new gateway JWT-verification middleware
(Step F) — including the fact that the gateway now constructs a
`jwt.Verifier` for the first time, which changes "where to look when a
token won't verify" for this specific path (gateway-side rejection,
before the request ever reaches the auth service).

## Tests

- Unit: OTP generation is cryptographically random and hashed correctly;
  `computeTrustLevel` against every combination of set/unset fields;
  `verification_codes`' upsert-on-resend behavior; the 60-second
  resend-cooldown check; the 5-attempt cap; free/role-based email domain
  rejection (already-specified lists from Verification Model § 5) rejects
  correctly and doesn't false-positive on a legitimate personal work
  address.
- Integration: full round trip against real Postgres for each verification
  path — phone (`LoggingSmsSender`, assert the logged code matches what
  verifies successfully), personal email (`LoggingEmailSender`, same
  assertion), corporate email (same, plus assert the raw email address is
  gone from `verification_codes` after success — grep the row, not just
  trust the code path).
- Test that a `GetProfile` response never contains a raw phone number or
  email address, only booleans/derived fields (Step E's design note) —
  this is a genuine PII-minimization property worth asserting directly,
  not just implementing correctly and hoping.

## Self-review checklist

- [ ] No raw phone number or email address ever appears in a log line,
      an error message returned to a client, or a `GetProfile` response —
      grep for `phone_number`/`personal_email`/`target` usage across every
      log/error/response-building call site.
- [ ] `verification_codes.code_hash` is genuinely a hash — grep for the
      raw code ever being stored or compared without hashing first.
- [ ] Every new `/v1/verification/*` and `/v1/users/me` route requires and
      correctly verifies the `Authorization: Bearer` header — try calling
      one with no header, an expired token, and another user's valid
      token, confirm all three fail correctly.
- [ ] The 60-second resend cooldown is enforced server-side, not just
      documented — try firing two `StartXVerification` calls back to back
      and confirm the second is rejected.
- [ ] `computeTrustLevel` is called and its result actually persisted
      after every relevant mutation — not just written as a function
      nobody calls.
- [ ] A verification-completing RPC actually returns a fresh access token
      reflecting the new trust level — decode it in a test, don't just
      trust the RPC succeeded.
- [ ] Corporate email's raw address is confirmed gone from
      `verification_codes` after successful verification (query it in a
      test after the fact).
- [ ] `computeTrustLevel` never returns 3 for a user missing any of
      phone/personal_email/legal_name/address — test this specific
      combination directly (work email verified, everything else unset),
      not just the "happy path all four then work email" case.
- [ ] The two-users-race-for-the-same-target case (Step F) maps to
      `ErrConflict` with a clear message, not an unhandled 500 or a
      silent overwrite.
- [ ] Gateway's new `jwt.Verifier` construction fails fast at startup if
      `JWT_PUBLIC_KEY_PATH` is missing/unreadable — same fail-fast
      discipline `config.go` already uses elsewhere, confirm it wasn't
      skipped for this specific new field.
- [ ] `backend/ARCHITECTURE.md` (Step I) actually updated — not just
      planned.
- [ ] `go vet`/`golangci-lint`/`go test` all clean; `docker compose up
      --build` works from a clean checkout with the new migration applied.
- [ ] Bring the diff back to Cowork for review before merging.

## Explicitly not in this addendum

Domain-age/SPF/DKIM/DMARC checks, the company verification database,
manual review for unknown corporate domains (all deferred per ADR-012),
KYC (Level 4), the 90-day work-email re-verification job itself (only its
data dependency, `work_email_verified_at`, is built here), phone/email as
an alternate login method, re-linking a different LinkedIn account to an
existing account, profile *editing* (changing an already-set legal
name/address/phone — this slice only covers first-time capture), and the
meetup-creation/scheduling feature entirely — that's a different domain
(ADR-008's "one service per bounded domain"), scoped as its own separate
ADR + PLAN.md once this slice lands, not bundled in here.
