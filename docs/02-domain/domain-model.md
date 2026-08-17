# Domain Model

Core entities and how they relate. This is the vocabulary every other document and, eventually, the codebase should use consistently.

## User

A person with an account. Has a **Trust Level** (see [Trust Levels](trust-levels.md)) and a continuous **trust score** (see [Trust & Safety Architecture](../03-architecture/trust-and-safety-architecture.md) § Continuous Trust Score) derived from verification state and behavior. Has zero or more **Verifications** (see [Verification Model](verification-model.md)): LinkedIn (federated or claimed), phone, personal email, personal details, professional/work email, and — optionally — KYC/biometric. Has zero or one active **Subscription** (see below). **Trust Level and Subscription are deliberately independent axes** — trust gates *what you're allowed to do* (safety), subscription gates *how much you can see / what convenience features you get* (monetization). Per the [Revenue Model](../01-product/revenue-model.md) guardrail, subscription must never buy trust: paying never grants a higher trust level, faster/skipped verification, or bypasses a trust-gated intent.

**Concrete schema for Level 2/3 verification ([ADR-012](../04-decisions/adr-012-level-2-3-verification-delivery-and-identity-anchors.md), 2026-08-17)**: `phone_number`/`personal_email` as nullable `UNIQUE` columns directly on `users` (presence = verified, same pattern as `linkedin_sub` — no separate boolean), `legal_name`/`address` self-reported with no verification, `company_domain`/`work_email_verified`/`work_email_verified_at` for corporate email (never the raw address, per [ADR-003](../04-decisions/adr-003-ephemeral-work-email-verification.md)). A separate `verification_codes` table holds pending OTP state (hashed code, expiry, attempt count) shared across all three OTP-based verifications — phone, personal email, and corporate email — one mechanism, three purposes. **Correction (2026-08-17)**: phone verification was originally scoped through Firebase Phone Auth; reconsidered and moved onto this same backend-owned OTP pattern instead (see [ADR-012](../04-decisions/adr-012-level-2-3-verification-delivery-and-identity-anchors.md)'s correction note) — no Firebase dependency anywhere in this slice. Execution detail in `backend/PLAN.md`/`frontend/PLAN.md`'s Level 2/3 addenda, not repeated here.

## Subscription

A User's paid-tier status: `free` / `premium` / `enterprise` (see [Revenue Model](../01-product/revenue-model.md)), plus status (`active`/`past_due`/`canceled`) and period dates. Modeled as its **own table with a foreign key to `users`**, not a boolean or enum column on the User record itself — the same reasoning as Verification being its own entity: a user's paid status changes independently of their identity record, can have its own history (upgrades, downgrades, billing events), and this shape is what lets a future paid tier (or per-feature entitlement, e.g. "search by company" as a standalone add-on) get added via a new row/table rather than a `users` schema migration. **Not yet built** — out of scope for the current LinkedIn-onboarding slice ([ADR-011](../04-decisions/adr-011-linkedin-onboarding-slice-design.md)), which intentionally ships only the columns Level 1a needs. Build this when Premium-tier work actually starts, not speculatively now.

## Company

An employer, tracked in the company-verification database ([Verification Model](verification-model.md) § Company Verification Database) with: official name, registration number, official domain + alternative domains, LinkedIn page, website, location, risk level, verification status. A User's work-email verification resolves to a `company_domain`, not a stored employer record tied to the raw email.

## Intent

The action a User currently wants: Ride Sharing, Coffee, Lunch, Drinks/Hangout, Event or Concert Companionship, Mentorship, Dating, etc. Intents are risk-tiered (low/medium/high — see [Safety Features Catalog](../03-architecture/safety-features-catalog.md) § Intent-Based Safety Rules), and each tier gates which safety controls and trust levels are required.

## Match

A mutual pairing of two Users on a compatible Intent, produced by the matching engine (FR-003 in [Requirements](../01-product/requirements.md)) from trust, location, availability, and compatibility signals (industry, interests, mutual communities).

## Meetup

A real-world meeting arising from a Match, gated by the **Safety Gate** flow (see [Safety UX Flows](../05-ux/safety-ux-flows.md)): match → intent selection → public-venue agreement → confirmation → safety checklist → optional live-location sharing → check-in timer → post-meetup confirmation from both parties. Feeds back into trust score.

## Ride (if/when ride-sharing ships)

A Meetup variant with its own driver/passenger verification requirements, vehicle data, live route tracking, and pickup-point rules — see [Safety Features Catalog](../03-architecture/safety-features-catalog.md) § Ride-Sharing Safety. Deferred past Phase 1 per [ADR-004](../04-decisions/adr-004-defer-dating-and-open-ride-sharing.md).

## Report

A user-initiated flag against another User or a Meetup/Ride, with a defined reason taxonomy (fake profile, scam, harassment, unsafe meetup, etc. — see [Safety Features Catalog](../03-architecture/safety-features-catalog.md) § Reporting and Blocking) and an SLA by severity (see [Operations & Incident Response](../03-architecture/operations-and-incident-response.md)).

## Trusted Contact

An emergency contact a User can optionally designate to automatically receive meetup/ride details, live location, and the matched peer's profile.

## Relationships (informal)

```
User ──has──> Trust Level, trust score
User ──has many──> Verification (LinkedIn federated/claimed, phone, personal email, personal details, work email, KYC)
User ──has zero-or-one active──> Subscription (tier, status — independent of Trust Level)
User ──resolves work email to──> Company (company_domain only, not raw email)
User ──selects──> Intent
Intent + Intent ──(compatibility engine)──> Match
Match ──(safety gate)──> Meetup / Ride
Meetup/Ride ──feeds──> trust score, Report (if something goes wrong)
User ──optionally designates──> Trusted Contact
```

## Related

[Trust Levels](trust-levels.md) · [Verification Model](verification-model.md) · [Requirements](../01-product/requirements.md) · [System Architecture](../03-architecture/system-architecture.md)
