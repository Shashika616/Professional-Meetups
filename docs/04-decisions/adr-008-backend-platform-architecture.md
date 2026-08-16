# [ADR-008](adr-008-backend-platform-architecture.md): Backend Platform — Go Microservices on Cloud Run, Custom Gateway, gRPC, Pub/Sub, Dedicated Realtime Gateway

**Status:** Accepted

## Context

Backend platform was the last major open item from [Project State](../00-project/project-state.md) § Blocked. Requirements: cost-conscious relative to AWS (Shashika's explicit reason for choosing GCP), a mobile-first app ([ADR-007 - Flutter as the Cross-Platform Frontend](adr-007-flutter-as-the-cross-platform-frontend.md)) with genuinely real-time surfaces (chat — FR-004, SOS/live-location — FR-006), and enough service isolation to support independent development/deployment as the system grows past onboarding.

Alternatives discussed and why they were or weren't chosen:
- **Kafka vs. GCP Pub/Sub** for async messaging — Pub/Sub chosen. Cloud Run is a request/response, scale-to-zero platform; it cannot host a persistent Kafka consumer process. Self-hosting Kafka would require GKE or Compute Engine, reintroducing the always-on infrastructure cost this whole platform choice was meant to avoid. Pub/Sub pushes directly to Cloud Run HTTP endpoints, fully managed, no broker to run.
- **REST vs. gRPC for interservice calls** — gRPC chosen, for direct request/response calls between services (e.g., gateway → auth service).
- **Google's managed "API Gateway" product vs. a custom gateway** — custom gateway chosen. The managed product is OpenAPI/REST-first and a poor fit for gRPC passthrough and WebSocket connections at the edge, both of which this system needs.
- **mTLS / service mesh vs. Cloud Run's IAM-based invoker auth** — IAM-based invoker auth chosen. Cloud Run has no sidecar-injected service mesh (unlike GKE + Anthos/Cloud Service Mesh) — there is no built-in X.509 mutual-TLS handshake between services. Cloud Run's alternative is IAM: each service has its own service account, calls carry a Google-signed OIDC identity token, and the receiving service's `roles/run.invoker` binding decides who may call it. This solves the actual problem (reject unauthorized callers) without a self-managed certificate authority. True mTLS would require moving to GKE + Cloud Service Mesh — a real platform change, not a config change — and isn't justified without a specific compliance driver naming X.509 mTLS.
- **WebSockets directly on each backend service vs. a dedicated realtime gateway** — dedicated service chosen. Cloud Run WebSocket connections pin to a single instance; since Cloud Run scales horizontally, a message published by one service has no way to reach a client whose socket happens to be held by a different instance of another service. A dedicated Realtime Gateway service holds all client connections itself and subscribes to the relevant Pub/Sub topics, fanning messages out only to the connections it holds — keeping the connection-scaling problem in exactly one place instead of spread across every service that needs to push to clients.

## Decision

- **Language/runtime**: Go, one Docker image per service, each deployed as an independent Cloud Run service (auth, matching, messaging, safety, admin, etc. — one per bounded domain).
- **Public entry point**: a custom Go gateway service, fronted by a Google External HTTPS Load Balancer (Serverless NEG → Cloud Run) for custom-domain TLS, or Cloud Run's own managed ingress if no custom domain is mapped yet. TLS termination is Google-managed either way — no service, including the gateway, handles raw certificates. The gateway owns JWT verification (public key only — see [ADR-009 - JWT Auth Strategy](adr-009-jwt-auth-strategy.md)), routing to the right backend service, and rate limiting.
- **Interservice synchronous calls**: gRPC.
- **Interservice async/eventing**: GCP Pub/Sub. Services publish domain events (e.g., `user.verified`, `match.created`, `meetup.confirmed`) to topics; other services subscribe as needed. No self-hosted message broker.
- **Realtime/WebSocket handling**: a dedicated Realtime Gateway service, separate from the public API gateway, holding client WebSocket connections and subscribing to relevant Pub/Sub topics to fan messages out to its own connected clients.
- **Service-to-service trust**: Cloud Run IAM invoker authentication (Google-signed OIDC identity tokens + `roles/run.invoker`), not self-managed mTLS.
- **Local development**: Postgres, Redis, and any other stateful dependency run as containers via `docker-compose.yml` — no native installs beyond Docker Desktop. A Pub/Sub local emulator substitutes for real Pub/Sub in dev.

## Consequences

- Fully serverless, scale-to-zero — matches the cost motivation that ruled out AWS in the first place; no idle infrastructure bill during pre-revenue/low-traffic periods.
- No message-broker ops burden (no Kafka to run, patch, or scale).
- The Realtime Gateway is a new, non-trivial service to design and build — it does not fall out of the other services for free, and it becomes the one place connection-scaling work concentrates.
- IAM-based invoker auth is materially simpler to operate than self-managed mTLS (no certificate rotation), at the cost of not being literal mTLS — acceptable unless a specific compliance requirement later names X.509 mTLS explicitly, at which point the affected services would need to move to GKE + Cloud Service Mesh.
- This is a real commitment to GCP specifically — Pub/Sub, Cloud Run IAM, and Cloud Run's ingress are all GCP-native. Migrating to another cloud later would mean re-architecting these three pieces, not just redeploying.

## Related

[System Architecture](../03-architecture/system-architecture.md) · [ADR-007 - Flutter as the Cross-Platform Frontend](adr-007-flutter-as-the-cross-platform-frontend.md) · [ADR-009 - JWT Auth Strategy](adr-009-jwt-auth-strategy.md) · [Requirements](../01-product/requirements.md) · [Project State](../00-project/project-state.md)
