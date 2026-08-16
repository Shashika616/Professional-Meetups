# Requirements

Source of truth for *what* we're building. See [Vision](../00-project/vision.md) for why, [Domain Model](../02-domain/domain-model.md) for what the terms mean, [System Architecture](../03-architecture/system-architecture.md) for how.

## Target audience

Working professionals, entrepreneurs, corporate employees, and university alumni who value networking and casual social connection but lack time or organic avenues to meet peers safely. See [Personas](personas.md) for the five core personas.

## Core business requirements

### Verification (BR-001 – BR-003b)

**Amended 2026-08-16 ([ADR-006](../04-decisions/adr-006-progressive-linkedin-first-trust-onboarding.md))** — verification is now progressive, not a single mandatory gate at signup. See [Verification Model](../02-domain/verification-model.md) and [Trust Levels](../02-domain/trust-levels.md) for full detail:

- **BR-001 — LinkedIn connection is the minimum bar to create an account.** LinkedIn OAuth/OpenID is preferred (Level 1a); a pasted profile URL is accepted as a lower-trust fallback (Level 1b) but does not unlock matching with other users.
- **BR-002 — Personal verification (phone OTP + personal email + personal name/address) is required to reach Level 2**, which is the real floor for matching and messaging with strangers.
- **BR-003 — Professional/corporate work email verification is optional**, not mandatory — it boosts trust to Level 3 and unlocks corporate-specific features (search by company, Enterprise participation). Free email domains (Gmail, Yahoo, etc.) are rejected as professional proof when a user does provide one.
- **BR-003b — Optional KYC/biometric liveness verification (Level 4)** may be introduced later, most likely as a prerequisite for dating or open ride-sharing, rather than at MVP.

### Matching (BR-004)

- **BR-004 — Intent-based matching, not profile browsing.** Users toggle a current intent (ride sharing, coffee, lunch, mentorship, dating). The matching engine evaluates trust, current location, destination, time availability, and compatibility (industry, interests, mutual communities) to surface relevant peers nearby.

### Revenue model (BR-005)

See [Revenue Model](revenue-model.md) for the three-tier breakdown (Free / Premium / Enterprise).

## Functional requirements

- **FR-001 — User Onboarding.** Progressive verification flow: LinkedIn connection (OAuth preferred, URL-paste fallback) to create an account; phone OTP + personal email + personal details to reach Level 2 and unlock matching; optional work-email domain validation and, later, optional KYC to reach Levels 3–4. See [Trust Levels](../02-domain/trust-levels.md).
- **FR-002 — Intent Dashboard.** Dynamic home screen where users select current intent and availability window.
- **FR-003 — Matching Algorithm.** Real-time geospatial + compatibility engine surfacing nearby users with matching intents.
- **FR-004 — Secure Messaging.** In-app chat with automated scam-pattern detection and link blocking for new accounts.
- **FR-005 — Meetup Safety Gate.** Mandatory checklist and public-venue recommendation system before a meetup is confirmed. See [Safety UX Flows](../05-ux/safety-ux-flows.md).
- **FR-006 — SOS / Emergency Flow.** Persistent panic button sharing live location with trusted contacts and local emergency services.
- **FR-007 — Enterprise Admin Panel.** Dashboard for HR to manage employee onboarding, verify corporate domains, and monitor internal commuting groups.

## Non-functional requirements

- **NFR-001 — Performance.** Matching engine returns results in under 200ms.
- **NFR-002 — Scalability.** Horizontal scaling to absorb concurrency spikes during peak commute hours.
- **NFR-003 — Privacy.** Exact user locations fuzzed (approximate) until a mutual meetup is confirmed. End-to-end encryption for all private messages.
- **NFR-004 — Compliance.** Full adherence to Sri Lanka's Personal Data Protection Act (PDPA) and global standards (GDPR). See [Sri Lanka PDPA - Work Email Verification](../07-research/sri-lanka-pdpa-work-email-verification.md).
- **NFR-005 — Availability.** 99.9% uptime with automated failover for safety/SOS-critical services.

## Explicitly out of scope for MVP

Dating mode, open ride-sharing, private-home meetups, global open registration, "search by company," premium visibility, and unrestricted event hosting are deferred past Phase 1 — see [Roadmap](../06-roadmap/roadmap.md) and [ADR-004](../04-decisions/adr-004-defer-dating-and-open-ride-sharing.md).

## Related

[Personas](personas.md) · [Competitive Landscape](competitive-landscape.md) · [Domain Model](../02-domain/domain-model.md) · [Trust Levels](../02-domain/trust-levels.md) · [System Architecture](../03-architecture/system-architecture.md)
