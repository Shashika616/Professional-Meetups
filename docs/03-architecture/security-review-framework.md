# Security Review Framework

Complements [Threat Model](threat-model.md) rather than duplicating it: the Threat Model is organized by attack *scenario* (fake identity, stolen accounts, social engineering...); this doc is organized by security *property*. Every non-trivial backend/frontend security review — mine or Claude Code's — should walk this checklist explicitly, not just "look for bugs." Introduced 2026-08-19 at Shashika's request; apply retroactively when reviewing existing slices too, not only new ones.

## Confidentiality — only authorized parties see the data

What this means concretely here: raw verification material stays scoped to the person it verifies, ratings stay anonymous to the ratee, secrets never leave the layer they belong to.

**Already enforced**: raw work-email address never persisted past the verification round-trip ([ADR-003](../04-decisions/adr-003-ephemeral-work-email-verification.md)); phone/personal email never returned in full over the API ([Verification Model](../02-domain/verification-model.md) § 1); ratings anonymous to the ratee, aggregate-only ([ADR-015](../04-decisions/adr-015-post-meetup-star-ratings.md)); LinkedIn access tokens and Apple/Google id_tokens verified then discarded, never persisted ([ADR-011](../04-decisions/adr-011-linkedin-onboarding-slice-design.md)/[ADR-014](../04-decisions/adr-014-federated-base-identity-optional-linkedin-age-eligibility.md)); `password_hash` has no field anywhere in `auth.proto` — confirmed by grep, it structurally cannot leak through any response, not just "the code happens not to send it."

**Open**: no field-level audit of *every* response message for over-fetching (e.g. does `ListMeetupRequests` ever return more than name/photo/trust-level pre-acceptance, as documented) — spot-checked repeatedly this session, not exhaustively swept once as its own pass.

## Integrity — data can't be modified in unauthorized or undetected ways

**Already enforced**: trust level computed server-side only (`computeTrustLevel`), never client-supplied; refresh-token rotation detects replay of an already-rotated token ([ADR-009](../04-decisions/adr-009-jwt-auth-strategy.md)); DB-level `CHECK`/`UNIQUE` constraints enforce invariants (self-rating blocked, one-rating-per-pair, capacity caps) rather than relying on application logic alone to hold; rating aggregate recomputed via `AVG()`/`COUNT()` in-transaction rather than an incremental update, avoiding drift ([ADR-015](../04-decisions/adr-015-post-meetup-star-ratings.md)); JWT signatures verified (our own tokens via `shared/jwt`, Apple/Google via JWKS) before any claim in them is trusted.

**Open**: none identified this pass beyond what's already tracked in Action Tracker (dead `crypto` package, uncommitted working tree as a data-loss-of-a-different-kind risk).

## Availability — the system keeps working, including under partial failure

**Already enforced**: every external HTTP call (LinkedIn, FCM, JWKS, Resend/Twilio) has an explicit timeout — the zero-value `http.Client` has none, which would let one hung dependency exhaust request-handling goroutines; the gateway's rate limiter **fails open** on a Redis error (confirmed by reading `rateLimited` directly — "rate limiting is defense in depth, not the only line of defense," so a Redis outage degrades protection, it doesn't take the app down); a failed *first* JWKS fetch for Apple/Google doesn't crash the auth service (`identity.go`'s `NoErrorReturnFirstHTTPReq` — a transient Apple/Google outage rejects federated logins, not LinkedIn/phone/email verification, which share nothing with it).

**Open, already tracked elsewhere, repeated here for completeness**: no CI/CD pipeline deploying the Go services; no Cloud Armor/WAF evaluation; RPO/RTO for Postgres not yet agreed despite backups/PITR being on ([ADR-010](../04-decisions/adr-010-postgresql-hosting-on-cloud-sql.md)); no cost/budget alerts.

## Authenticity — an entity really is who/what it claims to be

**Already enforced**: RS256 pinned on every third-party token verification (blocks `alg=none`/HS256-confusion forgery); issuer + audience checked, not just signature; LinkedIn's OAuth callback protected by `state`-based CSRF (`oauth_state.dart`) even without PKCE ([ADR-011](../04-decisions/adr-011-linkedin-onboarding-slice-design.md)'s documented reasoning for why the confidential-client secret substitutes for it there).

**Finding from this pass — real gap, not yet fixed**: the Apple/Google id_token verification (`identity.go`) checks signature, issuer, audience, and expiry, but never checks a `nonce` claim, and neither `signInWithApple`/`signInWithGoogle` (frontend) nor `Verify` (backend) generate or check one. Without it, a valid id_token intercepted within its (short) validity window — a compromised device, malware, a proxy — could in principle be replayed to `/v1/auth/federated/signup` to authenticate as that user. This is exactly what OIDC's `nonce` parameter exists to close, and there's already a working precedent for this exact shape of protection in this codebase: LinkedIn's `state`-based CSRF check. Low-cost fix (generate a nonce client-side per attempt, pass it into `getAppleIDCredential`/Google's authenticate call, verify it server-side against the token's `nonce` claim) — flagging rather than fixing now; say the word and I'll write the plan.

## Non-repudiation — a party can't credibly deny having done something

**Already enforced, incidentally rather than by deliberate design**: `meetup_safety_state`'s checklist-ack/check-in timestamps, `meetup_feedback.submitted_at`, `meetup_ratings.created_at`, refresh-token rotation's `replaced_by` chain — all timestamped, attributable to a specific `user_id`, and durable in Postgres.

**Finding from this pass**: there's no dedicated, tamper-evident audit log for security-sensitive actions as a deliberate concept — host accept/reject decisions, reports, any future admin/moderation action. What exists today is scattered timestamps on domain tables, useful for reconstructing *what* happened but not designed as an audit trail per se (no immutability guarantee beyond "nothing currently updates these columns," no admin-action log at all since there's no admin surface yet). Acceptable to leave as-is pre-launch — there's no moderation/admin feature built yet for it to cover — but worth deciding deliberately before [Safety Features Catalog](safety-features-catalog.md)'s reporting/moderation flow ships, rather than discovering the gap after reports start coming in.

## Authorization & accountability (the natural "and etc.")

Distinct from authenticity — not just "who are you" but "are you allowed to do *this specific thing*." **Already enforced**: server-side trust-level gating never trusts a client-supplied level (`middleware.TrustLevelFromContext`, sourced only from the verified JWT); every RPC takes `user_id` from the gateway's verified context, never the request body; `LinkIdentityToUser` hard-rejects cross-account collisions rather than silently merging ([ADR-014](../04-decisions/adr-014-federated-base-identity-optional-linkedin-age-eligibility.md)); rating submission checks participant status and confirmed attendance server-side, not client-trusted ([ADR-015](../04-decisions/adr-015-post-meetup-star-ratings.md)). This has been the single most consistently well-executed property across every review this session — worth naming as a strength, not just a checklist item to tick.

## Related

[Threat Model](threat-model.md) · [Trust & Safety Architecture](trust-and-safety-architecture.md) · [Privacy & Anti-Abuse Controls](privacy-and-anti-abuse-controls.md) · [ADR-003](../04-decisions/adr-003-ephemeral-work-email-verification.md) · [ADR-009](../04-decisions/adr-009-jwt-auth-strategy.md) · [ADR-011](../04-decisions/adr-011-linkedin-onboarding-slice-design.md) · [ADR-014](../04-decisions/adr-014-federated-base-identity-optional-linkedin-age-eligibility.md) · [ADR-015](../04-decisions/adr-015-post-meetup-star-ratings.md)
