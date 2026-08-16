# Domain Model

Core entities and how they relate. This is the vocabulary every other document and, eventually, the codebase should use consistently.

## User

A person with an account. Has a **Trust Level** (see [Trust Levels](trust-levels.md)) and a continuous **trust score** (see [Trust & Safety Architecture](../03-architecture/trust-and-safety-architecture.md) § Continuous Trust Score) derived from verification state and behavior. Has zero or more **Verifications** (see [Verification Model](verification-model.md)): LinkedIn (federated or claimed), phone, personal email, personal details, professional/work email, and — optionally — KYC/biometric.

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
User ──resolves work email to──> Company (company_domain only, not raw email)
User ──selects──> Intent
Intent + Intent ──(compatibility engine)──> Match
Match ──(safety gate)──> Meetup / Ride
Meetup/Ride ──feeds──> trust score, Report (if something goes wrong)
User ──optionally designates──> Trusted Contact
```

## Related

[Trust Levels](trust-levels.md) · [Verification Model](verification-model.md) · [Requirements](../01-product/requirements.md) · [System Architecture](../03-architecture/system-architecture.md)
