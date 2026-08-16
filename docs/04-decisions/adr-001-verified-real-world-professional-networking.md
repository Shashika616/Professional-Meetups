# [ADR-001](adr-001-verified-real-world-professional-networking.md): Build a Verified, Intent-Based, Real-World Professional Networking Platform

**Status:** Accepted

## Context

Market research ([Competitive Landscape](../01-product/competitive-landscape.md)) found no platform combining verified professional identity, real-world/location context, and instant intent-based matching. LinkedIn is async and online-only; ride-sharing apps extract no networking value; dating apps lack professional trust/verification; Meetup requires scheduling and targets groups.

## Decision

Build a platform where verified professionals toggle a current intent (ride sharing, coffee, lunch, mentorship, dating) and get matched with nearby, compatible, trust-appropriate peers in real time — merging networking, transportation, and social-serendipity into one product. See [Vision](../00-project/vision.md) and [Requirements](../01-product/requirements.md).

## Consequences

This is a much heavier trust/safety lift than a standard social or professional app, because it facilitates real-world, in-person meetings between strangers. It requires the full verification, trust-scoring, and safety-gate infrastructure documented in [Trust & Safety Architecture](../03-architecture/trust-and-safety-architecture.md), [Safety Features Catalog](../03-architecture/safety-features-catalog.md), and [Operations & Incident Response](../03-architecture/operations-and-incident-response.md) — these are not optional add-ons, they are load-bearing for the core value proposition.

## Related

[Vision](../00-project/vision.md) · [Competitive Landscape](../01-product/competitive-landscape.md) · [Requirements](../01-product/requirements.md)
