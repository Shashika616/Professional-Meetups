# Roadmap

Phased to reduce risk, not just to sequence features — see [ADR-004](../04-decisions/adr-004-defer-dating-and-open-ride-sharing.md) and [ADR-005](../04-decisions/adr-005-invite-only-company-based-closed-launch.md) for why dating/ride-sharing and open registration are deliberately late. Live status tracking lives in [Project State](../00-project/project-state.md); this note is the target sequence. Every backend item below should be built using [Feature Build Plan Template](feature-build-plan-template.md) (extracted from [ADR-011](../04-decisions/adr-011-linkedin-onboarding-slice-design.md)/`backend/PLAN.md`, the first slice) — scope boundary, test pyramid, and self-review checklist, not ad hoc.

## Phase 1 — Foundation & Safer MVP

- [ ] Core verification: phone OTP, LinkedIn OAuth, professional email verification ([Verification Model](../02-domain/verification-model.md)).
- [ ] Company allowlist + manual review for unknown domains.
- [ ] Intent dashboard with coffee, lunch, mentorship, professional-networking intents only.
- [ ] Full safety-gate infrastructure ([Safety UX Flows](../05-ux/safety-ux-flows.md)): safety checklist, public-venue recommendations, check-in, post-meetup feedback.
- [ ] Reporting/blocking, basic trust score, new-account restrictions.
- [ ] SOS/emergency flow.
- [ ] Launch closed/invite-only in Colombo, company-by-company ([ADR-005](../04-decisions/adr-005-invite-only-company-based-closed-launch.md)).

**Explicitly not in Phase 1**: dating, open ride sharing, private-home meetups, global open registration, search-by-company, premium visibility, unrestricted event hosting ([ADR-004](../04-decisions/adr-004-defer-dating-and-open-ride-sharing.md)).

## Phase 2 — Controlled Expansion

- [ ] Corporate onboarding at scale (SSO, HR-approved lists — see [Safety Features Catalog](../03-architecture/safety-features-catalog.md) § Enterprise mode).
- [ ] Interest-based communities.
- [ ] Verified events (with the fake-event controls in [Privacy & Anti-Abuse Controls](../03-architecture/privacy-and-anti-abuse-controls.md)).
- [ ] Ride-sharing — verified corporate groups only, full driver/vehicle verification stack ([Safety Features Catalog](../03-architecture/safety-features-catalog.md) § Ride-Sharing Safety).
- [ ] Mentorship marketplace.

## Phase 3 — Advanced AI & High-Trust Features

- [ ] AI personal assistant / advanced matching.
- [ ] Startup collaboration tools.
- [ ] Optional ID verification and background checks where legal (Trust Level 4, [Trust Levels](../02-domain/trust-levels.md)).
- [ ] Dating mode — highly restricted, opt-in, with the full control set in [Safety Features Catalog](../03-architecture/safety-features-catalog.md) § Dating Mode.
- [ ] Public (non-corporate) ride sharing, once moderation systems have matured under real usage.
- [ ] Consider transition from invite-only to open registration, contingent on trust/moderation metrics from [Operations & Incident Response](../03-architecture/operations-and-incident-response.md) § Safety dashboard.

## Related

[Requirements](../01-product/requirements.md) · [Vision](../00-project/vision.md) · [Project State](../00-project/project-state.md) · [Action Tracker](../00-project/action-tracker.md) · [Feature Build Plan Template](feature-build-plan-template.md) · [ADR-004](../04-decisions/adr-004-defer-dating-and-open-ride-sharing.md) · [ADR-005](../04-decisions/adr-005-invite-only-company-based-closed-launch.md)
