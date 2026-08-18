# Level 2/3 Verification Slice — Execution Plan

Built from `Feature Build Plan Template` (vault `06 - Roadmap`), same shape as the LinkedIn onboarding slice (ADR-011, `backend/PLAN.md`). Durable decisions already recorded in [ADR-012](../docs/04-decisions/adr-012-level-2-3-verification-delivery-and-identity-anchors.md) (once synced) — this file is disposable execution detail, same split as before.

## 1. Scope boundary

**In scope**: Level 2 (phone OTP, personal email OTP, personal details — legal name + address) and a deliberately MVP-scoped Level 3 (corporate email: OTP verification + domain capture only). All four steps must be independently completable and skippable — a user can finish onboarding at Level 1a and complete any subset of Level 2/3 later from `ProfilePage`.

**Explicitly NOT in scope, do not build**:
- The full Level 3 fraud-detection flow (domain-age/SPF/DKIM/DMARC checks, company verification database, manual review queue) — see ADR-012, this is a conscious, documented narrowing, not an oversight.
- Level 1b (pasted-URL LinkedIn fallback), Level 4 (KYC/liveness).
- Matching, messaging, meetups, the Safety Gate flow, or any Matching service — these depend on verified users existing, not the other way around, and are their own future initiative per [[Roadmap]]/[[Domain Model]].
- Re-linking a different LinkedIn account to an existing account (flagged open in ADR-012, still undecided).

If a design decision here seems to imply any of the above, stop and flag it rather than building toward it.

## 2. Prerequisites (human, not Claude Code)

1. **Firebase**: the project already has a live Firebase project (`professional-meetups-976d2`, used for `auth-bridge` hosting). Enable **Phone Authentication** in the Firebase console (Authentication → Sign-in method → Phone). Generate a **service account key** (Project Settings → Service Accounts → Generate new private key) for the Go backend to verify ID tokens server-side — store it the same way `secrets/jwt_private.pem` is handled (gitignored, mounted into the `auth` container, never committed). Add its path as a new env var, e.g. `FIREBASE_SERVICE_ACCOUNT_PATH`.
2. **Transactional email**: pick a provider for personal-email-OTP delivery (SendGrid or Postmark both have workable free tiers — either is fine, ADR-012 doesn't mandate one). Create an account, get an API key, add it as a new env var (e.g. `EMAIL_API_KEY`, `EMAIL_FROM_ADDRESS`). This blocks Step 3.4 below but nothing else — flag and stub behind a clear TODO if not available yet rather than blocking the whole slice.
3. Confirm the Level 1a stack still runs clean (`docker compose down -v && docker compose up --build`) before adding a new migration on top of it.

Claude Code should confirm both credentials are actually present and loadable via config before writing code that depends on them — same discipline as Step 0 of the Level 1a plan.

## 3. Build order

### 3.1 Migration — `db/migrations/0002_level2_3_verification.up.sql` (+ `.down.sql`)

Add to `users`:
```sql
phone_number         TEXT,
personal_email       TEXT,
full_legal_name      TEXT,   -- distinct from `full_name` (LinkedIn display name) — see note below
address_line         TEXT,   -- fuzzed/never shown to other users, same handling as location privacy
company_domain       TEXT,
work_email_verified  BOOLEAN NOT NULL DEFAULT false,
work_email_verified_at TIMESTAMPTZ,
```
Plus partial unique indexes (both nullable, mirroring `linkedin_sub`'s existing pattern):
```sql
CREATE UNIQUE INDEX idx_users_phone_number ON users (phone_number) WHERE phone_number IS NOT NULL;
CREATE UNIQUE INDEX idx_users_personal_email ON users (personal_email) WHERE personal_email IS NOT NULL;
```
**Never add a `work_email` (raw address) column** — ADR-003/ADR-012 require the raw corporate email is never persisted past the verification round trip. `company_domain`/`work_email_verified`/`work_email_verified_at` are the only durable record.

**`full_legal_name` is intentionally separate from `full_name`.** `full_name` is LinkedIn's display name (already exists, Level 1a); `full_legal_name` is the Level 2 personal-details capture, which may differ (nicknames, LinkedIn using a professional alias, etc.) and has different downstream uses (KYC document matching later, incident response) per [[Verification Model]] § 4 — don't collapse them into one column.

New table for OTP codes (used by both phone-confirmation-of-intent-to-verify and personal/corporate email — phone verification itself is delegated to Firebase and doesn't need a code stored here, only email does):
```sql
CREATE TABLE email_verification_codes (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    purpose      TEXT NOT NULL CHECK (purpose IN ('personal_email', 'corporate_email')),
    email_hint   TEXT,          -- for corporate_email, used to derive company_domain on success; never the final storage location
    code_hash    TEXT NOT NULL, -- SHA-256 of the 6-digit code, same non-reversible pattern as refresh_tokens.token_hash
    expires_at   TIMESTAMPTZ NOT NULL,
    consumed_at  TIMESTAMPTZ,
    attempt_count SMALLINT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_email_verification_codes_user_id ON email_verification_codes (user_id);
```
`attempt_count` exists so `VerifyCode` can reject after N (5) wrong attempts per code without a separate rate-limit lookup — cap it, don't just rely on the gateway's request-level rate limiter alone.

### 3.2 `shared/firebaseauth/` — new package

Thin wrapper around `firebase.google.com/go/v4/auth`: `VerifyIDToken(ctx, idToken string) (phoneNumber string, err error)`. Loads the service account key from `FIREBASE_SERVICE_ACCOUNT_PATH` (fail-fast at startup if missing, same pattern as `shared/jwt`'s key loading). Returns `apperror.ErrInvalidInput` on a token that fails verification or has no `phone_number` claim — never leak the underlying Firebase SDK error string to the client (same redaction discipline as the LinkedIn exchange, ADR-012/the recent security fix).

### 3.3 `shared/email/` — new package

`Sender` interface (`SendVerificationCode(ctx, to, code string) error`) with one real implementation against whichever provider was set up in Prerequisites. Interface-first specifically so tests use a fake, never the real provider (same pattern as `internal/linkedin.Client` in the Level 1a slice, which tests point at an `httptest.Server`).

### 3.4 `services/auth/internal/service/` additions

New methods on `Service`, each following the existing `CompleteLinkedInOnboarding`/`RefreshSession` shape (auth via the caller's existing valid access token — these are all authenticated actions on an already-logged-in user, unlike the LinkedIn flow which creates the session):

- `VerifyPhone(ctx, userID, firebaseIDToken string) error` — calls `firebaseauth.VerifyIDToken`, writes `phone_number` on success. Returns `apperror.ErrConflict` if the number is already claimed by a different user (the `UNIQUE` index will raise this — map it, don't let a raw constraint-violation string reach the client).
- `RequestEmailCode(ctx, userID, email, purpose string) error` — validates the email isn't already in use (personal_email) or isn't a free/role-based domain (corporate_email, reuse the validator pattern from `frontend/lib/core/validation/validators.dart`'s server-side mirror, port the same reject-list to Go), generates a 6-digit code, stores its hash + 10-minute expiry in `email_verification_codes`, sends via `shared/email`. Rate limit this specifically — reuse the existing Redis fixed-window limiter (`internal/middleware/ratelimit.go`'s pattern), scoped per-user this time, not just per-IP, since a logged-in user is the actual identity to throttle.
- `VerifyEmailCode(ctx, userID, code, purpose string) error` — looks up the most recent unconsumed code for `(userID, purpose)`, checks hash + expiry + `attempt_count < 5`, increments `attempt_count` on mismatch, marks `consumed_at` on success. On success: `purpose = personal_email` writes `personal_email`; `purpose = corporate_email` derives `company_domain` from the verified address's domain part, rejects if it's on the free-provider list (defense in depth — should already have been rejected at request time, but don't trust that alone), writes `company_domain`/`work_email_verified = true`/`work_email_verified_at = now()`. **The raw corporate email address is never written to `users` or logged** — it exists only transiently in the request and in `email_verification_codes.email_hint`, which should itself be cleared (set NULL) once consumed, not left sitting in the table.
- `SetPersonalDetails(ctx, userID, legalName, addressLine string) error` — straightforward write, no external verification (per [[Verification Model]] § 4, this is self-reported, cross-checked passively against other signals rather than independently verified).

### 3.5 `.proto` additions

New RPCs on `AuthService` mirroring the above 1:1 (`VerifyPhone`, `RequestEmailCode`, `VerifyEmailCode`, `SetPersonalDetails`), each taking the caller's `user_id` from the already-authenticated request context (gateway extracts this from the verified access token, same as any other authenticated endpoint — don't accept `user_id` as a client-supplied field on any of these, that would let a caller act on someone else's account). Regenerate via `buf`.

### 3.6 Gateway additions

New REST endpoints under `/v1/verification/*` (`phone`, `email/request-code`, `email/verify-code`, `personal-details`), all requiring a valid access token (existing auth middleware — check whether one exists yet from Level 1a; if not, this is the first endpoint that needs it, since `/v1/auth/*` endpoints are pre-session by definition). Translate gRPC responses/errors the same way `handlers.go` already does for the Level 1a endpoints.

## 4. Test strategy

- **Unit**: 6-digit code generation (cryptographically random, not `math/rand`), free/role-based email domain rejection list, `company_domain` extraction from a verified address, `attempt_count` cap logic.
- **Component (fake `firebaseauth`/`email.Sender`)**: `VerifyPhone` against a fake ID-token verifier (valid token → phone written; invalid/expired/wrong-claim token → rejected, no write); `RequestEmailCode`/`VerifyEmailCode` against a fake `Sender` (never call a real email API in tests).
- **Integration (real Postgres)**: the `UNIQUE` constraint on `phone_number`/`personal_email` actually rejects a second user claiming an already-verified value — this is exactly the kind of thing only a real DB constraint test catches.
- **Security-specific**: code guessing is rate-limited (both per-code `attempt_count` and the per-user Redis limiter); a consumed code can't be reused; an expired code is rejected; the raw corporate email address never appears in a log line or an error response (grep test, not just a functional one); phone/personal-email verification can't be used to overwrite another user's already-claimed value.

## 5. CI update

No new workflow needed — this extends `backend-ci.yml`'s existing Postgres-backed integration-test job with the new migration; confirm the CI job actually picks up `0002_level2_3_verification.up.sql` (it should, if it just runs all migrations in order, but verify rather than assume).

## 6. Self-review checklist

- [ ] Raw corporate email address never logged, never written to `users`, and `email_hint` is cleared after the code is consumed.
- [ ] Every new query is parameterized via `sqlc`.
- [ ] `user_id` on every new RPC comes from the authenticated request context, never a client-supplied field.
- [ ] Firebase ID token verification failures return a generic client-facing message, full detail logged server-side only (same redaction pattern as the recent LinkedIn-exchange security fix).
- [ ] Per-user rate limiting on `RequestEmailCode` is actually wired in, not just implemented.
- [ ] `UNIQUE` constraint violations on `phone_number`/`personal_email` map to a clear "already in use" client error, not a raw DB error leaking through.
- [ ] `go vet`/`golangci-lint` clean.
- [ ] `docker compose down -v && docker compose up --build` clean from a fresh checkout, migration included.
- [ ] Bring the diff back to Cowork for review before merging.

## Related

ADR-012 · ADR-006 · ADR-003 · `backend/PLAN.md` (Level 1a, same pattern) · `backend/ARCHITECTURE.md` (update after this lands — new `shared/firebaseauth`, `shared/email` packages, new endpoints) · `frontend/level2-3-verification-PLAN.md` (companion frontend plan, build after this)
