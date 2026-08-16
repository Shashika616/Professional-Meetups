# [ADR-011](adr-011-linkedin-onboarding-slice-design.md): LinkedIn Federated Onboarding Slice — PKCE, REST-to-gRPC Gateway Translation, Minimal Schema

**Status:** Accepted

## Context

First concrete backend slice to build: LinkedIn federated (Level 1a) onboarding per [ADR-006 - Progressive LinkedIn-First Trust Onboarding](adr-006-progressive-linkedin-first-trust-onboarding.md), on the platform decided in [ADR-008 - Backend Platform Architecture](adr-008-backend-platform-architecture.md) and [ADR-009 - JWT Auth Strategy](adr-009-jwt-auth-strategy.md). Full execution plan lives in the repo at `backend/PLAN.md` (implementation-detail-level, not durable architecture — this ADR records only the decisions that should outlive that specific build task).

## Decision

- **LinkedIn OAuth uses PKCE (RFC 7636)**, on top of this backend's own confidential-client `client_secret` exchange. The mobile app generates the `code_verifier`/`code_challenge` pair and never lets LinkedIn's authorization code be exchanged without it — this protects the code in transit specifically because a native mobile app is part of the flow, distinct from and in addition to the backend's own secret-based exchange with LinkedIn.
- **LinkedIn's own access token is never persisted.** The backend exchanges the code, calls LinkedIn's userinfo endpoint once, extracts `sub`/`name`/`picture`, and discards LinkedIn's token immediately — the same minimal-retention principle [ADR-003 - Ephemeral Work Email Verification](adr-003-ephemeral-work-email-verification.md) established for work email, applied here too.
- **The public API is REST/JSON; the gateway translates to gRPC internally.** Per [ADR-008](adr-008-backend-platform-architecture.md), gRPC is for interservice calls, not the public contract — mobile clients get a plain REST API (`POST /v1/auth/linkedin/callback`, `/v1/auth/refresh`, `/v1/auth/logout`), and the gateway is the only thing that speaks gRPC to the auth service.
- **Schema is scoped strictly to Level 1a.** `users` (id, linkedin_sub, full_name, profile_photo_url, headline, trust_level, account_status, timestamps) and `refresh_tokens` (hash-only, never the raw token) — no phone/personal-email/personal-details/corporate-email/KYC columns yet. `linkedin_sub IS NOT NULL` is the only signal needed for "this account completed federated LinkedIn" — no redundant boolean duplicating that fact.
- **Repository pattern + `sqlc`** for data access — interfaces in `internal/repository`, generated parameterized queries underneath, mirroring the same interface-first pattern already established in the Flutter frontend's `AuthService`/`MatchingService` contracts. SQL injection is eliminated structurally (no hand-built query strings), not by developer discipline.
- **Rate limiting on `/v1/auth/*`**: Redis-backed fixed-window counter, 20 requests/minute per IP per route. Deliberately the simplest correct algorithm, not a token bucket — upgrade only if the fixed-window edge effect actually becomes a measured problem.
- **`user.onboarded` Pub/Sub event** published on first-time onboarding, payload limited to `{user_id, trust_level, occurred_at}` — no name, photo, or other PII in the event, consistent with data-minimization applied everywhere else.

## Consequences

- PKCE adds a small amount of client-side complexity (the app must generate and hold the verifier for the duration of the flow) in exchange for meaningfully better protection against authorization-code interception — a good trade for a mobile-first app.
- Scoping the schema tightly to Level 1a means a near-certain future migration when Level 2 is built (adding phone/personal-email/personal-details columns) — accepted deliberately, per the same reasoning [ADR-008](adr-008-backend-platform-architecture.md)/[ADR-010](adr-010-postgresql-hosting-on-cloud-sql.md) already applied: add real schema for real built features, not speculative columns for unbuilt ones.
- The REST-to-gRPC translation boundary means every new backend capability needs a gateway handler *and* a service-side gRPC method, not just one or the other — slightly more code for the first slice, but keeps the public contract stable even if internal service boundaries move later.
- Execution detail (file layout, exact function signatures, step-by-step build order) intentionally lives in `backend/PLAN.md` in the repo, not here — that level of detail is disposable once the slice is built, while this ADR's decisions are meant to outlive it.

## Related

[ADR-006 - Progressive LinkedIn-First Trust Onboarding](adr-006-progressive-linkedin-first-trust-onboarding.md) · [ADR-008 - Backend Platform Architecture](adr-008-backend-platform-architecture.md) · [ADR-009 - JWT Auth Strategy](adr-009-jwt-auth-strategy.md) · [ADR-003 - Ephemeral Work Email Verification](adr-003-ephemeral-work-email-verification.md) · [System Architecture](../03-architecture/system-architecture.md) · [Project State](../00-project/project-state.md)
