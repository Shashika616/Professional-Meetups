# Project State

*Last updated: 2026-08-16. This is the one note that should change constantly — update it whenever the phase, decisions, or open questions shift. Everything else in this vault should change slowly.*

## Current phase

**Early build — frontend scaffold exists, no backend yet.** Code repo: `Professional-Meetups` (Flutter, mock services only), cloned locally, git-tracked, one commit ("Initial Frontend") plus uncommitted platform-scaffolding changes. Claude Code is wired up in VS Code (`/init` run, `CLAUDE.md` generated and now linked to this vault's exported `docs/`). iOS platform support was added locally via Xcode (confirmed target, [ADR-007](../04-decisions/adr-007-flutter-as-the-cross-platform-frontend.md)) alongside the existing Android/web targets; a `macos` folder also appeared as an incidental side effect and isn't an active target.

## Completed

- Initial market/problem/competitive research ([Competitive Landscape](../01-product/competitive-landscape.md), [Sources & Citations](../07-research/sources-and-citations.md)).
- Core business requirements drafted ([Requirements](../01-product/requirements.md)).
- Verification architecture and trust-level model designed ([Verification Model](../02-domain/verification-model.md), [Trust Levels](../02-domain/trust-levels.md)).
- Deep threat-modeling and trust & safety architecture pass ([Threat Model](../03-architecture/threat-model.md), [Trust & Safety Architecture](../03-architecture/trust-and-safety-architecture.md), [Safety Features Catalog](../03-architecture/safety-features-catalog.md), [Privacy & Anti-Abuse Controls](../03-architecture/privacy-and-anti-abuse-controls.md)).
- Legal review of work-email verification approach under Sri Lanka's PDPA ([Sri Lanka PDPA - Work Email Verification](../07-research/sri-lanka-pdpa-work-email-verification.md)).
- Seven foundational decisions recorded as ADRs (`04 - Decisions`), including the progressive LinkedIn-first trust-onboarding redesign ([ADR-006](../04-decisions/adr-006-progressive-linkedin-first-trust-onboarding.md)) and Flutter as the confirmed cross-platform frontend, Android + iOS ([ADR-007](../04-decisions/adr-007-flutter-as-the-cross-platform-frontend.md)).
- Added "The Overworked Professional" persona and sharpened the vault's professionals-only, safety-first positioning ([Vision](vision.md), [Personas](../01-product/personas.md)).
- Flutter mock-frontend code review (Cowork, 2026-08-16): overall well-structured (service-contract pattern, isolated Riverpod wiring, pure validators mirrored server-side, disciplined `mounted` checks). Findings: `IntentType`/`UserProfile` only model trust levels 1 and 4, no level 2/3 or federated-vs-claimed LinkedIn; a few hardcoded colors bypass `AppPalette`; most widget/feature files have no doc comments; test coverage thin (no tests for `MatchingService` or trust-gating logic).
- Added `.github/workflows/flutter-ci.yml` (format check + `flutter analyze --fatal-infos` + `flutter test --coverage` on push/PR) and untracked an accidentally-committed `run_log.txt`.
- Exported this vault's 28 notes into the repo at `docs/` (mirrored folder structure, wikilinks converted to relative markdown links) and updated the repo's `CLAUDE.md` to point at them, plus a "Known Gaps" section.

## Currently working on

- Nothing actively in progress. `onboarding_flow.dart` still implements the pre-[ADR-006](../04-decisions/adr-006-progressive-linkedin-first-trust-onboarding.md) trust model (phone → LinkedIn URL-paste → mandatory corporate email, sequential) — **known and deliberately not yet fixed**, flagged in the repo's `CLAUDE.md` "Known Gaps" section and here. Rebuild is planned to go through Claude Code in VS Code once picked up as a task, not through Cowork directly.

## Blocked / needs a decision from you

- **Level 1b product call**: confirm the [ADR-006](../04-decisions/adr-006-progressive-linkedin-first-trust-onboarding.md) rule that a pasted-URL-only LinkedIn ("claimed," not federated) cannot match or be matched — this was set as the default by Cowork per the safety-first principle, but it's a real product trade-off (it means some users who won't do LinkedIn OAuth can browse but never actually use the app) and deserves an explicit yes from you. Relevant now that `onboarding_flow.dart` is queued for a rebuild.
- **Legal sign-off**: [ADR-003](../04-decisions/adr-003-ephemeral-work-email-verification.md) and [Sri Lanka PDPA - Work Email Verification](../07-research/sri-lanka-pdpa-work-email-verification.md) flag that making *any* verification step mandatory as a condition of using the service should be reviewed by a Sri Lankan privacy lawyer before launch. [ADR-006](../04-decisions/adr-006-progressive-linkedin-first-trust-onboarding.md) makes this lower-stakes for work email specifically (now optional), but the baseline LinkedIn/personal-verification requirement would need its own version of this review if challenged.
- **Company allowlist ownership**: who manually curates/maintains the initial approved-company database (see [Verification Model](../02-domain/verification-model.md) § Company Verification Database)? Not yet assigned.
- **Tech stack (backend)**: still undecided — frontend is now fully confirmed (Flutter, Android + iOS + Web — [ADR-007](../04-decisions/adr-007-flutter-as-the-cross-platform-frontend.md)), but backend/database/cloud choices haven't been made. [System Architecture](../03-architecture/system-architecture.md) is backend-structure-only, not stack-specific there.
- **iOS permission strings**: `ios/Runner/Info.plist` has no usage-description keys yet (no location/camera plugins wired in yet). Needs `NSLocationWhenInUseUsageDescription`, `NSCameraUsageDescription`, etc. added — alongside the equivalent Android manifest permissions — before, not during, the SOS/live-location/KYC build-out. See [ADR-007](../04-decisions/adr-007-flutter-as-the-cross-platform-frontend.md).

## Next

1. Rebuild `onboarding_flow.dart` (+ `UserProfile`, `IntentType`) to match [ADR-006](../04-decisions/adr-006-progressive-linkedin-first-trust-onboarding.md)'s progressive trust model — via Claude Code, once queued.
2. Decide and record the backend technology stack (would become ADR-008+).
3. Begin closing the test-coverage gaps flagged in the code review (trust-gating logic, matching service).
4. Add iOS/Android permission usage-strings ahead of location/SOS/KYC feature work ([ADR-007](../04-decisions/adr-007-flutter-as-the-cross-platform-frontend.md)).
5. Continue Phase 1 build per [Roadmap](../06-roadmap/roadmap.md): safety-gate infrastructure, coffee/lunch/drinks/event-companionship/mentorship intents.

## Change log

- **2026-08-16** — Vault created and populated from initial product/architecture/legal research docs.
- **2026-08-16** — Trust-level model redesigned to progressive/LinkedIn-first onboarding per [ADR-006](../04-decisions/adr-006-progressive-linkedin-first-trust-onboarding.md); updated [Trust Levels](../02-domain/trust-levels.md), [Verification Model](../02-domain/verification-model.md), [Requirements](../01-product/requirements.md), [Domain Model](../02-domain/domain-model.md), [Vision](vision.md), [Personas](../01-product/personas.md) accordingly.
- **2026-08-16** — Connected the `Professional-Meetups` Flutter repo; reviewed the mock frontend; added CI workflow; exported this vault to the repo's `docs/` and linked it from `CLAUDE.md`, including a flagged onboarding-flow/trust-model gap.
- **2026-08-16** — iOS platform support confirmed in the repo (Xcode/`flutter create` scaffolding, `macos` folder appeared incidentally); recorded as [ADR-007](../04-decisions/adr-007-flutter-as-the-cross-platform-frontend.md); updated [System Architecture](../03-architecture/system-architecture.md), repo `CLAUDE.md`, and this note accordingly.
