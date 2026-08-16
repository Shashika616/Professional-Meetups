# Project State

*Last updated: 2026-08-16. This is the one note that should change constantly — update it whenever the phase, decisions, or open questions shift. Everything else in this vault should change slowly.*

## Current phase

**Pre-build / product discovery.** No code repository exists yet. This vault (product requirements, domain model, architecture, ADRs) is the only artifact so far.

## Completed

- Initial market/problem/competitive research ([Competitive Landscape](../01-product/competitive-landscape.md), [Sources & Citations](../07-research/sources-and-citations.md)).
- Core business requirements drafted ([Requirements](../01-product/requirements.md)).
- Verification architecture and trust-level model designed ([Verification Model](../02-domain/verification-model.md), [Trust Levels](../02-domain/trust-levels.md)).
- Deep threat-modeling and trust & safety architecture pass ([Threat Model](../03-architecture/threat-model.md), [Trust & Safety Architecture](../03-architecture/trust-and-safety-architecture.md), [Safety Features Catalog](../03-architecture/safety-features-catalog.md), [Privacy & Anti-Abuse Controls](../03-architecture/privacy-and-anti-abuse-controls.md)).
- Legal review of work-email verification approach under Sri Lanka's PDPA ([Sri Lanka PDPA - Work Email Verification](../07-research/sri-lanka-pdpa-work-email-verification.md)).
- Six foundational decisions recorded as ADRs (`04 - Decisions`), including the progressive LinkedIn-first trust-onboarding redesign ([ADR-006](../04-decisions/adr-006-progressive-linkedin-first-trust-onboarding.md)).
- Added "The Overworked Professional" persona and sharpened the vault's professionals-only, safety-first positioning ([Vision](vision.md), [Personas](../01-product/personas.md)).

## Currently working on

- Nothing implementation-side yet. Next concrete step is standing up the git repository and bootstrapping `CLAUDE.md` from [CLAUDE.md Template (For Code Repo)](claude-md-template-for-code-repo.md).

## Blocked / needs a decision from you

- **Level 1b product call**: confirm the [ADR-006](../04-decisions/adr-006-progressive-linkedin-first-trust-onboarding.md) rule that a pasted-URL-only LinkedIn ("claimed," not federated) cannot match or be matched — this was set as the default by Cowork per the safety-first principle, but it's a real product trade-off (it means some users who won't do LinkedIn OAuth can browse but never actually use the app) and deserves an explicit yes from you.
- **Legal sign-off**: [ADR-003](../04-decisions/adr-003-ephemeral-work-email-verification.md) and [Sri Lanka PDPA - Work Email Verification](../07-research/sri-lanka-pdpa-work-email-verification.md) flag that making *any* verification step mandatory as a condition of using the service should be reviewed by a Sri Lankan privacy lawyer before launch. [ADR-006](../04-decisions/adr-006-progressive-linkedin-first-trust-onboarding.md) makes this lower-stakes for work email specifically (now optional), but the baseline LinkedIn/personal-verification requirement would need its own version of this review if challenged.
- **Company allowlist ownership**: who manually curates/maintains the initial approved-company database (see [Verification Model](../02-domain/verification-model.md) § Company Verification Database)? Not yet assigned.
- **Tech stack**: no frontend/backend/database/cloud choices have been made yet. [System Architecture](../03-architecture/system-architecture.md) is currently structure-only, not stack-specific.

## Next

1. Decide and record the technology stack (would become ADR-007+).
2. Stand up the git repository; add `CLAUDE.md` and `docs/` (see [CLAUDE.md Template (For Code Repo)](claude-md-template-for-code-repo.md)).
3. Begin Phase 1 build per [Roadmap](../06-roadmap/roadmap.md): onboarding + verification, intent dashboard, safety-gate infrastructure, coffee/lunch/drinks/event-companionship/mentorship intents only.

## Change log

- **2026-08-16** — Vault created and populated from initial product/architecture/legal research docs.
- **2026-08-16** — Trust-level model redesigned to progressive/LinkedIn-first onboarding per [ADR-006](../04-decisions/adr-006-progressive-linkedin-first-trust-onboarding.md); updated [Trust Levels](../02-domain/trust-levels.md), [Verification Model](../02-domain/verification-model.md), [Requirements](../01-product/requirements.md), [Domain Model](../02-domain/domain-model.md), [Vision](vision.md), [Personas](../01-product/personas.md) accordingly.
