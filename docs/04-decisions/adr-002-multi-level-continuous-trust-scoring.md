# [ADR-002](adr-002-multi-level-continuous-trust-scoring.md): Layered Trust Levels (0–4) Plus a Continuous Trust Score, Not Binary Verified/Unverified

**Status:** Accepted

## Context

No single verification method (phone, LinkedIn, work email, profile details, profile photo) is individually reliable — each has a documented bypass ([Verification Model](../02-domain/verification-model.md) § Core principle). A binary "verified / not verified" flag would let an attacker who defeats one method pass as fully trusted.

## Decision

Use a five-tier discrete Trust Level (0 Unverified → 4 High-Trust, see [Trust Levels](../02-domain/trust-levels.md)) combined with a continuously updated trust score built from positive and negative behavioral signals ([Trust & Safety Architecture](../03-architecture/trust-and-safety-architecture.md) § Continuous Trust Score). Feature access — matches, intents available, messaging limits, search, ride-sharing/dating eligibility — is gated by both together, not by a one-time pass/fail check.

## Consequences

Adds real engineering complexity: a trust-scoring engine, per-feature gating logic, and an internal safety dashboard ([Operations & Incident Response](../03-architecture/operations-and-incident-response.md)) to monitor it. In exchange, the platform degrades gracefully under attack (a compromised single signal doesn't grant full trust) and can tighten or loosen access dynamically without a hard ban being the only lever. New accounts are throttled by default regardless of verification completeness (see [Trust & Safety Architecture](../03-architecture/trust-and-safety-architecture.md) § New-account slowdown).

## Related

[Trust Levels](../02-domain/trust-levels.md) · [Verification Model](../02-domain/verification-model.md) · [Trust & Safety Architecture](../03-architecture/trust-and-safety-architecture.md)
