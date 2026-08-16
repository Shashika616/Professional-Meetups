# System Architecture

**Status: frontend and backend platform both confirmed; no backend code exists yet.** Frontend is Flutter, targeting Android and iOS as the two primary mobile platforms (web secondary) — see [ADR-007](../04-decisions/adr-007-flutter-as-the-cross-platform-frontend.md). Backend is Go microservices on Cloud Run — see [ADR-008](../04-decisions/adr-008-backend-platform-architecture.md) (platform), [ADR-009](../04-decisions/adr-009-jwt-auth-strategy.md) (auth), [ADR-010](../04-decisions/adr-010-postgresql-hosting-on-cloud-sql.md) (database hosting). Per-service data-model choices are still made incrementally as each service gets designed (see [Project State](../00-project/project-state.md) § Next); this note captures the shape the backend must have to satisfy [Requirements](../01-product/requirements.md). Several real production concerns are confirmed as **not yet decided** — see [Project State](../00-project/project-state.md) § Backend platform — open technical items — this is a known, tracked gap, not an oversight.

## Frontend (confirmed — [ADR-007](../04-decisions/adr-007-flutter-as-the-cross-platform-frontend.md))

Flutter, single codebase → Android + iOS + Web. State management is Riverpod; navigation is a fixed linear flow via `Navigator.push`/`pushReplacement`, no router package. Business logic is isolated behind `abstract interface class` service contracts (`AuthService`, `MatchingService`, etc.) with `Mock*` implementations for the current backend-less scaffold — this is the seam a real backend integration swaps into, without touching UI code. Cross-platform permission handling (location, camera, for SOS/KYC features) needs to be declared independently on both iOS (`Info.plist` usage-description keys) and Android (manifest permissions) before those features are implemented — see [ADR-007](../04-decisions/adr-007-flutter-as-the-cross-platform-frontend.md) § Consequences.

## Backend platform (confirmed — [ADR-008](../04-decisions/adr-008-backend-platform-architecture.md), [ADR-009](../04-decisions/adr-009-jwt-auth-strategy.md))

Go microservices, each an independent Cloud Run deployment. A custom Go gateway (behind a Google Load Balancer for TLS) is the single public entry point — JWT verification, routing, rate limiting. Interservice synchronous calls use gRPC; async/eventing uses GCP Pub/Sub (not Kafka — Cloud Run can't host a persistent consumer). A dedicated Realtime Gateway service holds WebSocket connections and fans out Pub/Sub events to them, since Cloud Run WebSocket connections pin to a single instance — **the exact mechanism for routing a message to the specific gateway instance holding a given user's socket (e.g. a Redis-backed `user_id → instance_id` registry) is not yet designed**, only the general shape (dedicated service + Pub/Sub subscription). Service-to-service trust is Cloud Run's IAM invoker auth (Google-signed OIDC identity tokens), not a self-managed mTLS mesh. Auth: RS256 JWT access tokens (~15 min) plus rotated refresh tokens, stored in OS secure storage on mobile and httpOnly cookies on the (future) web admin dashboard; staff/admin auth is a fully separate system from consumer auth.

**Database**: Cloud SQL for PostgreSQL (Enterprise edition, shared-core tier, single-zone to start, automated backups + point-in-time recovery from day one) — not Cloud Run, which is stateless and cannot host a database itself; not AlloyDB, whose premium isn't justified at this scale yet. See [ADR-010](../04-decisions/adr-010-postgresql-hosting-on-cloud-sql.md). Cloud Run services connect via the Cloud SQL Auth Proxy. Unlike the rest of this stack, Cloud SQL is **not scale-to-zero** — it's a constant cost from provisioning, not just from traffic.

**Local dev**: Postgres/Redis/Pub-Sub-emulator all run via `docker-compose`, no native installs beyond Docker Desktop.

## Components implied by requirements (not yet built)

- **Onboarding & verification service** — phone OTP, LinkedIn OAuth, work-email domain verification, device attestation. See [Verification Model](../02-domain/verification-model.md).
- **Trust engine** — computes trust level (0–4) and continuous trust score; gates feature access. See [Trust Levels](../02-domain/trust-levels.md), [Trust & Safety Architecture](trust-and-safety-architecture.md).
- **Matching engine** — real-time geospatial + compatibility matching on Intent. Must return results in <200ms (NFR-001).
- **Messaging service** — in-app chat with automated scam/risk detection, link blocking for new/low-trust accounts. See [Safety Features Catalog](safety-features-catalog.md) § Communication Safety.
- **Meetup/Safety-gate service** — orchestrates the pre-meetup checklist, public-venue suggestions, check-in timers, and post-meetup feedback. See [Safety UX Flows](../05-ux/safety-ux-flows.md).
- **SOS/emergency service** — panic button, trusted-contact notification, location-based emergency-number lookup.
- **Company/domain intelligence service** — domain age/DNS/MX/SPF-DKIM-DMARC checks, company allowlist, manual-review queue for unknown domains.
- **Moderation/anti-abuse service** — device fingerprinting, ban-evasion detection, message risk scanning, organized-crime graph analysis. See [Privacy & Anti-Abuse Controls](privacy-and-anti-abuse-controls.md).
- **Enterprise admin panel** — HR-facing dashboard for employee onboarding, domain verification, commuting-group monitoring.
- **Internal safety dashboard** — for the human review/trust & safety team. See [Operations & Incident Response](operations-and-incident-response.md).

## Non-functional constraints that shape architecture

- **NFR-001 Performance** — matching engine <200ms → likely needs a fast geospatial index, not a naive DB query per match.
- **NFR-002 Scalability** — horizontal scaling for commute-hour spikes → stateless services, queue-backed matching where possible.
- **NFR-003 Privacy** — locations must be fuzzed until mutual confirmation; E2EE on private messages → architecture must support message-content opacity to the backend where feasible, or at minimum strict access control + audit logging.
- **NFR-004 Compliance** — PDPA/GDPR → minimal-retention data model, especially for work-email verification (see [ADR-003](../04-decisions/adr-003-ephemeral-work-email-verification.md)) and sensitive documents (see [Privacy & Anti-Abuse Controls](privacy-and-anti-abuse-controls.md) § Do Not Store Unnecessary Sensitive Documents).
- **NFR-005 Availability** — 99.9% uptime with automated failover specifically for SOS/emergency paths — this likely needs its own resilience budget separate from the rest of the system.

## Build & CI/CD pipelines

- **Frontend CI (built, running)**: `.github/workflows/flutter-ci.yml` — `dart format --set-exit-if-changed`, `flutter analyze --fatal-infos`, `flutter test --coverage`, on every push/PR to `main`/`develop`. Runs from `frontend/` since the repo's reorganization into `frontend/` + `backend/` (see [Project State](../00-project/project-state.md)). Flutter version is deliberately pinned, not `channel: stable`, after an earlier unpinned-channel formatting break.
- **Backend local build/verification (in progress)**: not a CI/CD pipeline — Claude Code, supervised, running `backend/PLAN.md` locally: `docker compose up --build` (multi-stage Dockerfiles — compile in a full `golang:*-alpine` image, ship only the static binary in a minimal `gcr.io/distroless/static-debian12:nonroot` runtime image, keeping the production image small and free of build tools, per [ADR-008](../04-decisions/adr-008-backend-platform-architecture.md)'s cost model), `go build`/`go test`, and an integration test against a real Postgres container. One-time write→build→read-output→fix→retest loop, not an automated on-push pipeline.
- **Backend CI/CD (planned, not yet built)**: `backend/PLAN.md` Step 6 has Claude Code add `backend-ci.yml` — checkout → pinned `setup-go` → `go build ./...` → `go vet ./...` → `golangci-lint run` → `go test ./...` with a `postgres:16-alpine` service container. Kept as its own workflow file, not merged with `flutter-ci.yml` — unrelated toolchains. Tracked as an open item in [Project State](../00-project/project-state.md) § Backend platform until it actually exists.

## Data sensitivity note

Two data classes need architecturally distinct handling: (1) short-lived verification material (raw work email, ID photos, vehicle documents) that should be deleted or heavily restricted post-verification, and (2) long-lived low-sensitivity derived data (company_domain, trust score, verification booleans) that's safe to keep. Don't let them live in the same table/service with the same retention policy.

## Related

[Requirements](../01-product/requirements.md) · [Domain Model](../02-domain/domain-model.md) · [Trust & Safety Architecture](trust-and-safety-architecture.md) · [Project State](../00-project/project-state.md)
