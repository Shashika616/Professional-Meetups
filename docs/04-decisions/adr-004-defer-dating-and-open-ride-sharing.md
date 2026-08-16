# [ADR-004](adr-004-defer-dating-and-open-ride-sharing.md): Defer Dating Mode and Open Ride-Sharing to Later Phases; Phase 1 Ships Low-Risk Intents Only

**Status:** Accepted

## Context

Ride-sharing and dating are the two highest-risk intents in the product (see [Threat Model](../03-architecture/threat-model.md) § F, G and [Safety Features Catalog](../03-architecture/safety-features-catalog.md) § Ride-Sharing Safety, § Dating Mode). Each needs materially heavier safety infrastructure than networking intents: ride-sharing needs driver/vehicle/license verification and live route tracking; dating needs age gating, opt-in visibility, sextortion/romance-scam detection, and a much higher trust-level floor. Launching either at day one, before the trust and safety-gate infrastructure exists, would expose users to the platform's most serious threat categories (physical safety, financial fraud, sexual exploitation) with the least mature defenses.

## Decision

Phase 1 launches with professional verification, coffee/lunch/networking/mentorship intents, public-meetup recommendations, reporting/blocking, and a basic trust score only. Dating and open ride-sharing are explicitly deferred — not cut — to Phase 2 (ride-sharing, for verified corporate groups only) and Phase 3 (dating, plus ID verification and background checks where legal). See [Roadmap](../06-roadmap/roadmap.md) for the full phase breakdown.

## Consequences

Slower path to full feature parity with the original vision, but drastically reduces launch-time risk in the categories most likely to cause real physical or financial harm — and gives the trust-scoring and moderation systems time to mature (and gather real usage signal) before they're asked to gate the riskiest intents. Also aligns with [ADR-005](adr-005-invite-only-company-based-closed-launch.md)'s closed-growth strategy: a smaller, more homogenous early user base is exactly the environment where it's safe to introduce ride-sharing/dating later.

## Related

[Threat Model](../03-architecture/threat-model.md) · [Safety Features Catalog](../03-architecture/safety-features-catalog.md) · [Roadmap](../06-roadmap/roadmap.md) · [ADR-005](adr-005-invite-only-company-based-closed-launch.md)
