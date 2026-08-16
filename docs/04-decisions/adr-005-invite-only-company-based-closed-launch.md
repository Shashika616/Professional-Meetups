# [ADR-005](adr-005-invite-only-company-based-closed-launch.md): Launch Closed and Invite-Only (Colombo, Company/Community-Based) Rather Than Open Public Registration

**Status:** Accepted

## Context

[Operations & Incident Response](../03-architecture/operations-and-incident-response.md) § "The strongest defense: closed, trusted growth" identifies invite-only/company-based launch as the single strongest lever for reducing fake accounts and abuse — stronger than any individual verification control. Sri Lanka's professional ecosystem is small and community-driven enough to make this practical rather than growth-limiting.

## Decision

Phase 1 beta is restricted to Colombo and selected professional communities, onboarded company-by-company, with the top companies manually pre-verified ([Verification Model](../02-domain/verification-model.md) § Company Verification Database). Access is granted via invite codes, corporate invitations, university alumni networks, professional associations, chamber-of-commerce partnerships, tech-community partnerships, and a manually-approved waitlist — not open public sign-up. Positioning: "join through your company or a verified member."

## Consequences

Slower initial user growth than an open launch, but a materially stronger trust foundation to build on, a cleaner path to Enterprise-tier adoption (companies are already the onboarding unit), and a marketing narrative (exclusivity via verified community) that doubles as a safety feature. Public open registration becomes a Phase 3+ consideration, gated on the moderation and trust systems having matured under real usage. See [Roadmap](../06-roadmap/roadmap.md).

## Related

[Operations & Incident Response](../03-architecture/operations-and-incident-response.md) · [Verification Model](../02-domain/verification-model.md) · [Roadmap](../06-roadmap/roadmap.md) · [ADR-004](adr-004-defer-dating-and-open-ride-sharing.md)
