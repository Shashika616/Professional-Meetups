# [ADR-006](adr-006-progressive-linkedin-first-trust-onboarding.md): Progressive, LinkedIn-First Trust Onboarding — Corporate Email and KYC Become Optional Boosters, Not Mandatory Gates

**Status:** Accepted — amends the original Level 1 definition in [Trust Levels](../02-domain/trust-levels.md) and the "three mandatory pillars" framing in BR-001–003 of [Requirements](../01-product/requirements.md).

## Context

The original design ([ADR-002](adr-002-multi-level-continuous-trust-scoring.md), initial [Trust Levels](../02-domain/trust-levels.md) draft, initial [Requirements](../01-product/requirements.md) BR-001–003) required phone OTP, LinkedIn OAuth, and professional/corporate email verification all completed before a user reached even "Level 1 — Basic Verified." Shashika's refined product direction (2026-08-16) instead wants a lower-friction, progressive onboarding built around this scenario: a busy corporate employee wants safe company for coffee, drinks, or a concert — not a dating app, not an anonymous stranger. The safety guarantee that makes that safe to offer is that the person on the other end has *some* verifiable professional footprint, escalating as needed:

1. Start with LinkedIn — federated (OAuth) if possible, or a pasted profile URL as a fallback ("not secure," in Shashika's words).
2. Increase trust by adding personal email, phone number, and personal details (name, address).
3. Optionally add corporate/work email for a further boost.
4. If needed later, introduce KYC — live selfie/liveness — to confirm the account belongs to a real human.

## Decision

Replace the flat "verify everything up front" model with five progressive levels (full definitions in [Trust Levels](../02-domain/trust-levels.md)):

- **Level 0** — Unverified, onboarding only.
- **Level 1** — LinkedIn Connected. **1a** federated (OAuth, preferred) or **1b** claimed (pasted URL, self-reported). This is the minimum bar to create an account.
- **Level 2** — Personal Verified: phone OTP + personal email + personal details (name, address). **This is the real floor for matching and messaging with strangers**, not Level 1.
- **Level 3** — Corporate Verified: work/corporate email or enterprise SSO. Optional; boosts trust and unlocks corporate-specific features.
- **Level 4** — KYC / High-Trust: live liveness check, optional ID. Optional for MVP; likely a prerequisite before enabling dating or open ride-sharing ([ADR-004](adr-004-defer-dating-and-open-ride-sharing.md)).

**Explicit product rule**: Level 1b (pasted LinkedIn URL) accounts cannot match or be matched with other users. A pasted URL is a claim, not a verification, and the platform's entire safety premise — "the person you're meeting has a real professional footprint" — would break if an unverified claim alone unlocked meeting strangers. This restriction is a deliberate architectural guardrail, flagged here explicitly per the Cowork/Claude Code operating charter's rule against silently deciding things with significant safety consequences; it's adjustable, but the default is conservative on purpose.

## Consequences

- **Lower signup friction**: an account can exist, and a user can browse the app, after LinkedIn alone (even the unverified-URL path) — better for activation than the original three-pillar hard gate.
- **The professional-only safety promise now rests on Level 2, not on signup.** Marketing and in-app copy should be precise about this: "verified professional" as a user-facing claim should mean Level 2+, not merely "connected a LinkedIn."
- **Corporate email is no longer mandatory.** This actually *helps* the open legal question flagged in [ADR-003](adr-003-ephemeral-work-email-verification.md) and [Sri Lanka PDPA - Work Email Verification](../07-research/sri-lanka-pdpa-work-email-verification.md) — the PDPA consent-conditionality concern was specifically about making a broader-than-necessary requirement mandatory as a condition of service; making work-email verification optional removes that tension for work email specifically. The mandatory-condition question for baseline LinkedIn/personal-detail verification would need its own (likely much weaker) version of that legal review if it ever comes up.
- **KYC is explicitly deferred**, not designed away — [Verification Model](../02-domain/verification-model.md) § 7 keeps the full liveness/ID control set ready to activate before Level 4 becomes load-bearing for dating or ride-sharing.
- Requires updating: [Trust Levels](../02-domain/trust-levels.md) (done), [Verification Model](../02-domain/verification-model.md) (done), [Requirements](../01-product/requirements.md) BR-001–003 (done), [Domain Model](../02-domain/domain-model.md) Intent/Verification entities (done), [Vision](../00-project/vision.md) positioning language (done), [Personas](../01-product/personas.md) (added "The Overworked Professional" persona reflecting the scenario that motivated this change).

## Related

[Trust Levels](../02-domain/trust-levels.md) · [Verification Model](../02-domain/verification-model.md) · [Requirements](../01-product/requirements.md) · [Vision](../00-project/vision.md) · [ADR-002](adr-002-multi-level-continuous-trust-scoring.md) · [ADR-003](adr-003-ephemeral-work-email-verification.md) · [ADR-004](adr-004-defer-dating-and-open-ride-sharing.md)
