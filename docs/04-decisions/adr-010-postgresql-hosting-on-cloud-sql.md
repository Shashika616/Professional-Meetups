# [ADR-010](adr-010-postgresql-hosting-on-cloud-sql.md): PostgreSQL Hosting — Cloud SQL for PostgreSQL (Enterprise), Not AlloyDB, Not Self-Hosted

**Status:** Accepted

## Context

[ADR-008 - Backend Platform Architecture](adr-008-backend-platform-architecture.md) established Postgres running via `docker-compose` for local development but didn't decide production hosting. Cloud Run itself cannot host a database — it's stateless and containers are recycled between requests, so anything written to local disk doesn't persist. GCP offers two managed Postgres-compatible services:

- **Cloud SQL for PostgreSQL** — the standard managed offering, two editions (Enterprise, the baseline; Enterprise Plus, higher-performance machine types at a premium).
- **AlloyDB for PostgreSQL** — Google's newer, higher-performance, Postgres-*compatible* database aimed at heavy analytical/transactional workloads at scale, carrying roughly a 30–40% price premium over Cloud SQL's tiers ([Bytebase: Understanding Google Cloud AlloyDB Pricing](https://www.bytebase.com/blog/understanding-google-alloydb-pricing/), [Bytebase: AlloyDB vs. Cloud SQL](https://www.bytebase.com/blog/alloydb-vs-cloudsql/)).

Cloud Spanner also exists (globally-distributed, horizontally-scalable, Postgres-compatible interface) but solves a scale problem far beyond anything this app needs at MVP.

## Decision

- **Cloud SQL for PostgreSQL, Enterprise edition**, smallest shared-core tier to start.
- **Single-zone (no HA) initially** — regional HA is a configuration change to enable later, not a migration, so this isn't a one-way door.
- **Automated backups and point-in-time recovery enabled from day one**, regardless of tier or traffic — this is cheap insurance and shouldn't wait for "real" launch.
- **Cloud Run connects via the Cloud SQL Auth Proxy** (IAM-authenticated, no manual TLS certificate management), not a private VPC connection — revisit only if latency measurements actually justify the added networking complexity.
- **Region** matches wherever the Cloud Run deployment region lands (still open — see [Project State](../00-project/project-state.md)).
- AlloyDB and Spanner remain available upgrade paths later, not a starting point — no reason to pay their premium for a scale problem that doesn't exist yet.

## Consequences

- Unlike Cloud Run and Pub/Sub, Cloud SQL is **not scale-to-zero** — it's a constant, always-on cost from the day it's provisioned, regardless of traffic. This is a deliberate, necessary exception to the "pay only for what you use" pattern the rest of [ADR-008](adr-008-backend-platform-architecture.md)'s stack follows, and worth remembering when reviewing the GCP bill.
- Backups/PITR being on from day one reduces data-loss risk but does **not** by itself define a recovery-time or recovery-point objective — RPO/RTO targets still need to be explicitly set as a business decision, not just inferred from "backups are on." Tracked as an open item.
- Deferring HA to a later flip-of-a-switch keeps early cost down at the price of a single point of failure until that switch is flipped — acceptable pre-launch, worth revisiting before any real user data is at stake.

## Related

[ADR-008 - Backend Platform Architecture](adr-008-backend-platform-architecture.md) · [System Architecture](../03-architecture/system-architecture.md) · [Project State](../00-project/project-state.md)
