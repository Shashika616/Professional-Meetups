# Domain Model

Core entities and how they relate. This is the vocabulary every other document and, eventually, the codebase should use consistently.

## User

A person with an account. Has a **Trust Level** (see [Trust Levels](trust-levels.md)) and a continuous **trust score** (see [Trust & Safety Architecture](../03-architecture/trust-and-safety-architecture.md) § Continuous Trust Score) derived from verification state and behavior. Has zero or more **Verifications** (see [Verification Model](verification-model.md)): LinkedIn (federated or claimed), phone, personal email, personal details, professional/work email, and — optionally — KYC/biometric. Has zero or one active **Subscription** (see below). **Trust Level and Subscription are deliberately independent axes** — trust gates *what you're allowed to do* (safety), subscription gates *how much you can see / what convenience features you get* (monetization). Per the [Revenue Model](../01-product/revenue-model.md) guardrail, subscription must never buy trust: paying never grants a higher trust level, faster/skipped verification, or bypasses a trust-gated intent.

**Base identity ([ADR-014](../04-decisions/adr-014-federated-base-identity-optional-linkedin-age-eligibility.md), 2026-08-19, final shape)**: account creation has four parallel paths — Sign in with Apple, Sign in with Google, email+password (OTP-verified), or LinkedIn direct — not exclusively LinkedIn. The first three land at [Trust Levels](trust-levels.md)' Level 0 (a real, persisted account, not a pre-signup placeholder); LinkedIn grants Level 1 immediately. Every User also has a mandatory, one-time **age eligibility confirmation**: `age_confirmed_over_18` (boolean, self-attested) + `age_confirmed_at` — deliberately no date of birth stored, same minimal-retention reasoning as [ADR-003](../04-decisions/adr-003-ephemeral-work-email-verification.md).

Schema: a `user_identities` table (`user_id, provider, subject, email, linked_at`, unique on `(provider, subject)`) holds Apple/Google identities only — `linkedin_sub` stays on `users` directly, whether set at direct signup or connected later, for one consistent home regardless of when it was linked. A nullable `password_hash` on `users` backs the email+password path.

**Identity resolution ([ADR-014](../04-decisions/adr-014-federated-base-identity-optional-linkedin-age-eligibility.md))** is the load-bearing rule keeping four entry paths from producing duplicate or hijacked accounts: every federated login (Apple/Google/LinkedIn) always resolves-or-creates against the same lookup regardless of which screen triggered it; linking a new identity to an already-authenticated account hard-rejects if that identity already belongs to someone else, never merges; and an email+password signup against an email that's already a verified `personal_email` elsewhere is treated as account recovery (new password on the existing account), not a duplicate. Genuinely unlinkable duplicates (e.g. separate Apple- and Google-created accounts for the same person, no shared verified identifier) are accepted, not solved — mitigated by every account being independently capped at its own trust level, not by detection.

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

**Revised 2026-08-17 ([ADR-013](../04-decisions/adr-013-host-initiated-meetup-scheduling-with-join-requests.md))**: a Meetup is now **host-initiated**, not solely derived from a Match. A User creates one directly — Intent, a time window, location (address + lat/lng, picked via Mapbox search/autocomplete or pin-drop — switched from the originally-planned Google Maps, [ADR-013](../04-decisions/adr-013-host-initiated-meetup-scheduling-with-join-requests.md) §4), and a participant capacity — then other Users send **Meetup Requests** to join it (see below). The eventual compatibility-matching engine (Phase 3, [Roadmap](../06-roadmap/roadmap.md)) layers in later as a discovery/ranking aid that surfaces relevant open Meetups to a User, not as the thing that creates a Meetup — Match→Meetup (this doc's pre-2026-08-17 model) is superseded as the primary Phase 1 path, though the Match entity itself stays defined below for that future use.

**Revised again 2026-08-20 ([ADR-016](../04-decisions/adr-016-meetup-time-windows-and-host-initiated-lifecycle-closing.md))**: timing is a real `window_start`/`window_end` range, not a single instant — every Meetup requires one, "today" included (previously a null-means-now special case with no time entry at all, a reported gap). Displayed on every meetup card, not just stored. Lifecycle is now tracked explicitly: **Open** (`open`/`full`) vs. **History** (`completed`/`cancelled`), with the host able to manually **Close** a Meetup once its window has started — reviving `meetup_status`'s `completed` value, which [ADR-015](../04-decisions/adr-015-post-meetup-star-ratings.md) originally left deliberately unused. This is purely organizational/display — it does not gate or interact with rating eligibility (still per-participant, see Rating below).

Every Meetup that reaches a confirmed participant (i.e. at least one Meetup Request accepted) is gated by the **Safety Gate** flow (see [Safety UX Flows](../05-ux/safety-ux-flows.md)), unchanged in substance: safety checklist → optional live-location sharing → check-in timer → post-meetup confirmation from all confirmed participants (a "did it happen" choice plus an optional free-text note, [ADR-016](../04-decisions/adr-016-meetup-time-windows-and-host-initiated-lifecycle-closing.md) — the confirmation's real wall-clock timestamp was already captured independent of the scheduled window since [ADR-013](../04-decisions/adr-013-host-initiated-meetup-scheduling-with-join-requests.md)'s original migration). Feeds back into trust score. Capacity is a hard cap — once reached, further requests are auto-rejected ([ADR-013](../04-decisions/adr-013-host-initiated-meetup-scheduling-with-join-requests.md)), not waitlisted.

Trust gating for hosting/requesting: per-intent via `IntentType.requiredTrustLevel`, with the default floor raised from Level 1 to Level 2 for coffee/lunch/networking/mentorship (ride-share/dating stay at Level 4) — see [Trust Levels](trust-levels.md) for why Level 1 alone isn't the floor for real stranger interaction.

## Meetup Request

**New 2026-08-17 ([ADR-013](../04-decisions/adr-013-host-initiated-meetup-scheduling-with-join-requests.md)).** A User's request to join an open Meetup they didn't create. Fields: requesting User, target Meetup, status (`pending` / `accepted` / `rejected` / `withdrawn`), created/resolved timestamps. The host sees each requester's name and trust level (never raw contact details pre-acceptance) and accepts or rejects individually; both the requester and, on acceptance, the wider request pool (if capacity is now full) get notified. A request auto-resolves to `rejected` with a "meetup full" reason if capacity fills before the host acts on it.

## Rating

**New 2026-08-19 ([ADR-015](../04-decisions/adr-015-post-meetup-star-ratings.md)).** A 1-5 star score one participant gives another after a shared Meetup, unlocked the moment the *rater* confirms attendance via their own **Meetup Request** feedback (`happened: true`) — not the Meetup's own Open/History lifecycle state ([ADR-016](../04-decisions/adr-016-meetup-time-windows-and-host-initiated-lifecycle-closing.md)). A host closing a Meetup has no effect on rating eligibility either way; the two are deliberately decoupled. One Rating per (Meetup, rater, ratee) pair, immutable. Anonymous to the ratee: a User only ever sees their own aggregate (`rating_average`, `rating_count` on User), never which participant gave which score — a known, accepted limitation of that anonymity in very small meetups, where a single rating can only have come from one other person. Ratable set is a Meetup's host plus its accepted requesters, excluding self; a participant doesn't need their own confirmed attendance to be rated by someone who did attend. Separate, additive signal from Trust Level — never replaces or is replaced by verification badges.

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
User ──creates (Level 2+, per-intent)──> Meetup (intent, timing, location, capacity)
User ──sends──> Meetup Request ──(host accepts/rejects)──> Meetup
Intent + Intent ──(compatibility engine, Phase 3)──> Match ── (future: surfaces/ranks open Meetups)
Meetup ──(safety gate, on first accepted request)──> active Meetup / Ride
Meetup/Ride ──feeds──> trust score, Report (if something goes wrong)
User ──confirms attendance, then rates──> other participants ──(anonymous aggregate)──> User.rating_average/rating_count
User ──optionally designates──> Trusted Contact
```

## Related

[Trust Levels](trust-levels.md) · [Verification Model](verification-model.md) · [Requirements](../01-product/requirements.md) · [System Architecture](../03-architecture/system-architecture.md) · [ADR-014](../04-decisions/adr-014-federated-base-identity-optional-linkedin-age-eligibility.md) · [ADR-015](../04-decisions/adr-015-post-meetup-star-ratings.md) · [ADR-016](../04-decisions/adr-016-meetup-time-windows-and-host-initiated-lifecycle-closing.md)
