# Onboarding Slice — Execution Plan

Give this whole file to Claude Code as the task brief. It's written to be
followed literally, in order. Scaffolding (docker-compose, Dockerfiles, SQL
migrations, the .proto contract, go.work/go.mod files) already exists —
this plan is for the Go source code, which doesn't exist yet.

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
3. LinkedIn redirects to `redirect_uri` (a custom URL scheme deep link back
   into the app, e.g. `professionalconnections://auth/linkedin/callback`)
   with `code` and `state`.
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
  body:  {"authorization_code": "...", "code_verifier": "...", "redirect_uri": "..."}
  200:   {"user_id": "...", "access_token": "...", "refresh_token": "...",
          "expires_in": 900, "is_new_user": true}
  400:   invalid/expired code, PKCE mismatch
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
(Level 2), corporate email (Level 3), KYC (Level 4), the matching engine,
messaging, the Realtime Gateway, and the API Gateway's Cloud Run/Load
Balancer deployment manifests (Cloud Run YAML / Terraform — this plan only
covers the code and local Docker setup). Flag rather than build if asked to
extend into any of these without an explicit go-ahead.
