# Trust Levels

Users are not treated equally — access is gated by a discrete trust level plus a continuous trust score layered on top (see [Trust & Safety Architecture](../03-architecture/trust-and-safety-architecture.md) § Continuous Trust Score). Recorded as a deliberate design choice in [ADR-002](../04-decisions/adr-002-multi-level-continuous-trust-scoring.md).

**Why this exists at all**: this app connects working professionals with strangers for real, in-person meetings — coffee, drinks, a concert, a ride — precisely the kind of casual social contact that's easy for a fraudster, thief, or manipulator to exploit. We are responsible for every person's safety the moment we introduce them to someone. Trust levels are the mechanism for that responsibility: nobody gets access to matching with strangers without first clearing a bar we control, and the bar gets higher as the intent gets riskier.

**Revised 2026-08-16** ([ADR-006](../04-decisions/adr-006-progressive-linkedin-first-trust-onboarding.md)) to a progressive, LinkedIn-first onboarding model — this supersedes the original "three mandatory pillars" framing. See [ADR-006](../04-decisions/adr-006-progressive-linkedin-first-trust-onboarding.md) for the full rationale.

## Level 0 — Unverified / Restricted

Has only installed the app or started signup. Allowed: view limited onboarding, complete verification. Not allowed: match, chat, see other users, use intent-based discovery.

## Level 1 — LinkedIn Connected

The minimum bar to create an account. Two paths, not treated as equally trustworthy:

- **1a — Federated (preferred).** LinkedIn OAuth/OpenID login. The platform confirms the user actually controls the LinkedIn account. Allowed: browse, complete profile, limited discovery within low-risk intents only (coffee, lunch, networking), basic messaging with heavy new-account restrictions (see [Trust & Safety Architecture](../03-architecture/trust-and-safety-architecture.md) § New-account slowdown).
- **1b — Claimed (fallback, "not secure").** User pastes their LinkedIn profile URL instead of connecting via OAuth — used when federated login isn't available or the user opts out of it. This is a **self-reported claim, not a verification**: anyone can paste any public URL. **Product rule: Level 1b accounts cannot match or be matched with other users.** They can complete their profile and view the app, but must upgrade to 1a (OAuth) or reach Level 2 before unlocking any real interaction with a stranger. This restriction is deliberate — see [ADR-006](../04-decisions/adr-006-progressive-linkedin-first-trust-onboarding.md) for why we didn't let a pasted URL alone unlock matching.

Restricted at all of Level 1: no dating, no ride sharing, no high-risk intents, no search by company, no bulk visibility.

## Level 2 — Personal Verified

Adds, on top of Level 1a: phone OTP verification, personal email verification, and personal details (legal name, address) captured and verified — see [Verification Model](verification-model.md). **This is the real floor for interacting with strangers.** Allowed: standard matching and messaging, medium-risk intents (coffee, lunch, drinks, casual hangouts, event/concert companionship), user ratings, community participation.

## Level 3 — Corporate Verified

Optional trust booster on top of Level 2: verified corporate/work email ([Verification Model](verification-model.md) § Professional email verification), or — for B2B customers — verification via company SSO, corporate directory, HR-approved employee list, or enterprise admin verification. Not required to use the app, but unlocks: search-by-company, corporate community features, internal networking, safe commuting, cross-team collaboration, and is a strong positive trust-score signal. This is the tier the Enterprise revenue tier depends on — see [Revenue Model](../01-product/revenue-model.md).

## Level 4 — KYC / High-Trust (optional, introduce if/when needed)

Not required for MVP. For users who want, or the platform requires for the highest-risk intents (dating, ride sharing, late-night or private meetups), a KYC-style check proving the account belongs to a real, live human: government ID verification, face liveness check ("live pics"), video verification, optional address proof, background check where legally allowed, driver/vehicle verification for ride sharing. **This should stay optional and privacy-sensitive** — no sensitive documents collected unless necessary, encrypted, legally compliant, and securely stored (see [Privacy & Anti-Abuse Controls](../03-architecture/privacy-and-anti-abuse-controls.md)). Revisit making this mandatory before enabling dating or open ride-sharing ([ADR-004](../04-decisions/adr-004-defer-dating-and-open-ride-sharing.md)).

## Design note

A verified badge should never be read by users as "this person is safe" — only "this account passed certain checks." Always pair verification UI with the caveat that verification does not guarantee good intentions. See [Safety Features Catalog](../03-architecture/safety-features-catalog.md) § Do Not Over-Trust "Verified" Badges.

## Related

[Verification Model](verification-model.md) · [Domain Model](domain-model.md) · [Trust & Safety Architecture](../03-architecture/trust-and-safety-architecture.md) · [ADR-002](../04-decisions/adr-002-multi-level-continuous-trust-scoring.md) · [ADR-006](../04-decisions/adr-006-progressive-linkedin-first-trust-onboarding.md) in `04 - Decisions`
