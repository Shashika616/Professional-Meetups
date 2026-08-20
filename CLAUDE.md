# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository layout

This is a monorepo with two independent projects, reorganized 2026-08-16 for a clean split as the backend was added:

```
Professional-Meetups/
├── frontend/     # the Flutter app — see "Commands" and "Architecture" below
├── backend/      # Go microservices — see backend/README.md and backend/PLAN.md
├── docs/         # generated product/architecture docs, applies to both sides
├── scripts/      # sync_docs_from_vault.py
└── CLAUDE.md     # this file
```

`docs/` stays at the repo root deliberately — it's product/architecture truth that applies to both `frontend/` and `backend/`, not code belonging to either. Each of `frontend/` and `backend/` has its own `.gitignore` for toolchain-specific artifacts (Dart/Flutter build output vs. Go build output); the root `.gitignore` only covers repo-wide/editor/OS concerns.

## Project Documentation

Business/product truth for this app lives in `docs/` (exported from the Cowork product/architecture vault, not written by hand — see `docs/00-project/cowork-operating-charter.md` for how the two tools divide labor). Read the relevant doc before making a significant architecture or business-rule change; don't re-derive requirements from the UI code.

- `docs/00-project/vision.md`, `docs/00-project/project-state.md` — what we're building, and current status/open questions.
- `docs/01-product/requirements.md`, `docs/01-product/personas.md` — functional/non-functional requirements, target users.
- `docs/02-domain/domain-model.md`, `docs/02-domain/trust-levels.md`, `docs/02-domain/verification-model.md` — the domain vocabulary and the trust-level/verification model the app implements.
- `docs/03-architecture/` — system architecture, threat model, trust & safety architecture, safety features catalog, privacy/anti-abuse controls, operations & incident response.
- `docs/04-decisions/` — ADR-001 through ADR-012. Why decisions were made; don't silently re-decide something recorded here — if a decision looks wrong once you're in the code, say so and propose a new ADR rather than implementing around it.
- `docs/05-ux/safety-ux-flows.md`, `docs/06-roadmap/roadmap.md`, `docs/07-research/` — UX flows, phased roadmap, legal/market research backing the above.

`docs/` is generated — never hand-edit files under it. The vault ("Professional Meetups Vault" in the user's Documents folder) is the source of truth; after editing a note there, run `python3 scripts/sync_docs_from_vault.py` from the repo root to refresh `docs/` (mechanical wikilink-to-relative-link conversion, no content changes — safe and cheap to run anytime).

**Check `docs/00-project/action-tracker.md` at the start of any nontrivial task.** It's the current, actively-maintained checklist of what's outstanding (pending decisions, human-prerequisite steps not yet done, deliberately-deferred gaps) — more current than this file's own Known Gaps section below, which can drift between vault syncs.

**Also check `TESTING-NOTES.md` at the repo root before treating any verification or map/location behavior as production-real.** It tracks active testing-only shortcuts (currently: a hardcoded OTP bypass, and a provisional Stadia Maps API key standing in for an undecided final map provider) that must not ship. Unlike `docs/`, this file is not vault-generated — it's a live working note, edit it directly when adding or removing a shortcut.

## Known Gaps

- **Level 1a (LinkedIn federated onboarding) — fixed (2026-08-16/17).** `onboarding_flow.dart` runs a real LinkedIn OAuth/OIDC flow against the live backend (ADR-011), replacing the old mandatory phone → LinkedIn-URL-paste → corporate-email wizard. PKCE was later removed from this flow (LinkedIn's self-serve product doesn't support it — see ADR-011's correction section); `state`-based CSRF protection remains.
- **Level 2/3 (phone/personal-email/personal-details, corporate email MVP) — built and committed (2026-08-18, commit `b94ad1d`).** See ADR-012 and its correction section for the design decisions actually implemented (phone verification uses the same backend-owned OTP mechanism as email, not Firebase; Twilio for SMS, Resend for email, both with logging fallbacks when credentials are empty). Level 1b (pasted-URL LinkedIn fallback) and Level 4 (KYC) remain unbuilt and out of scope — don't build UI or backend for either without an explicit go-ahead.
- **Meetup scheduling (ADR-013) — built, not yet committed, as of 2026-08-18.** Host-initiated meetups with join requests (a new `services/meetup` backend, `Meetup`/`MeetupRequest` in `docs/02-domain/domain-model.md`), superseding Match→Meetup as the Phase 1 mechanism. `IntentType.requiredTrustLevel`'s default is now **2**, not 1 (mirrored server-side in `services/meetup/internal/service/trustgate.go` — a change to one side must be made on the other too). Full Safety Gate wired in. The mock `MatchesPage`/`MockMatchingService`/`MatchProfile` are deleted, not kept in parallel — `matches_page.dart` is now the real browse-open-meetups surface, wired to a real `MeetupService`. **This whole slice is uncommitted** — don't assume it's on `origin/develop` yet.
- **Active testing-only shortcuts — see `TESTING-NOTES.md` at the repo root.** A hardcoded OTP bypass and a provisional (not final) map/location provider choice. Must not ship either.

## Commands

All Flutter commands run from `frontend/` (the Flutter project root — `cd frontend` first, or pass `--target`/set your editor's working directory there). The repo root itself is just the monorepo wrapper; `pubspec.yaml` lives at `frontend/pubspec.yaml`, not at the repo root.

```bash
cd frontend
flutter pub get                        # install dependencies
flutter run                            # run on a connected device/emulator (android, ios, or chrome)
flutter run -d chrome                  # run the web target specifically
flutter run -d ios                     # run on an iOS simulator/device
flutter test                           # run all tests
flutter test test/validators_test.dart # run a single test file
flutter analyze                        # static analysis (uses analysis_options.yaml / flutter_lints)
dart format --output=none --set-exit-if-changed .  # formatting check (CI-enforced, see below)
dart format .                          # auto-fix formatting
flutter build apk                      # android release build
flutter build ios                      # ios release build (requires signing set up in Xcode first)
flutter build web                      # web release build
```

`android`, `ios`, `macos`, and `web` platform folders all exist (added 2026-08-16 via Xcode/`flutter create` platform scaffolding) — there is no linux/windows scaffolding. iOS is a real target (built alongside Android per the product's cross-platform requirement); the `macos` folder was scaffolded as a side effect and isn't an active target unless that changes.

iOS-specific notes:
- Open `frontend/ios/Runner.xcworkspace` (not `.xcodeproj`) in Xcode for signing/capabilities — CocoaPods wires the Flutter framework into the workspace, not the bare project.
- `frontend/ios/Runner/Info.plist` has no usage-description keys yet (no location/camera/photo-library permissions requested) because no plugin needs them yet. Before implementing anything that touches live location, SOS, or KYC selfie capture (`docs/03-architecture/safety-features-catalog.md`, `docs/02-domain/verification-model.md` § 7), add the corresponding `NSLocationWhenInUseUsageDescription` / `NSCameraUsageDescription` etc. here, and the matching Android manifest permissions — both platforms need this before those features can run at all, not just before they can ship.
- `frontend/analysis_options.yaml` excludes `build/**`, `android/**`, `ios/**`, `macos/**`, `web/**`, `windows/**`, `linux/**` from the analyzer — generated platform code won't show up in `flutter analyze` or the CI lint step.

`.github/workflows/flutter-ci.yml` runs on every push/PR to `main`/`develop`: `dart format --set-exit-if-changed`, `flutter analyze --fatal-infos`, then `flutter test --coverage`, all with `frontend/` as the working directory. Run all three locally (from `frontend/`) before pushing.

## Architecture

The Flutter app under `frontend/` started as a frontend-only scaffold with every service as a `Mock*` implementation. **As of ADR-011, `AuthService` is real** (`HttpAuthService`, talking to the live `backend/` gateway) — `MockAuthService` is kept only for widget tests, not used at runtime. **As of ADR-013, `MeetupService` is also real** (`HttpMeetupService`) — the old `MatchingService`/`MockMatchingService`/`MatchProfile` are deleted, not kept as a parallel mock. The codebase is deliberately structured so each mock can be swapped for a real implementation independently, one bounded slice at a time, without touching unrelated UI code — that's still the pattern for whatever's next (messaging, SOS, etc.), it's just that fewer things are still mocked than when this section was first written. Everything below is relative to `frontend/`.

### State management: Riverpod

All app-wide state and dependency wiring lives in [frontend/lib/core/providers/app_providers.dart](frontend/lib/core/providers/app_providers.dart). Services are exposed as plain `Provider`s (e.g. `authServiceProvider`, `meetupServiceProvider`) — `authServiceProvider`, `meetupServiceProvider`, and `tokenRefresherProvider` are bound to real implementations; anything not yet backed by its own slice stays on a `Mock*`. UI reads state via `ConsumerWidget`/`ConsumerStatefulWidget` and `ref.watch`/`ref.read`; there's no other state management approach in the app.

### Service-contract pattern

Business logic is defined as `abstract interface class` contracts in `frontend/lib/core/services/` (`AuthService`, `MatchingService`), each with a `Mock*` implementation. Doc comments on these interfaces state the operating principle explicitly: **the client never decides trust/validity — it only displays what the server would return**. Mock implementations intentionally throw `FormatException` to simulate server-side rejection. When adding new backend-touching features, follow this same pattern: define the interface, mock it, wire it through `app_providers.dart`.

### Validation

`frontend/lib/core/validation/validators.dart` holds pure, static validation functions (phone, OTP, LinkedIn URL, corporate email incl. free-provider/role-based-mailbox rejection). These exist for UX speed only — every mock service re-checks the same rules to simulate the server being the actual source of truth. Keep this split when extending: pure validators for instant UI feedback, service-layer checks for the "server" decision.

### Navigation

No router package — plain `Navigator.push`/`pushReplacement` with `MaterialPageRoute`. Fixed linear flow:

```
main.dart → SplashScreen → (session found) → AppShell
                          → (no session) → LandingPage → OnboardingFlow (LinkedIn sign-in;
                            Level 2/3 steps land here, built as of 2026-08-18)
                          → AppShell (bottom nav: Home, Matches, Safety, Chats, Profile)
```

(Superseded 2026-08-17: this was a 4-step phone/LinkedIn/corporate-email wizard with no session persistence — replaced by ADR-011's real LinkedIn OAuth flow plus session-restore-on-launch.)

The "Matches" tab (`matches_page.dart`) is, despite its class/file name kept for minimal navigation churn (ADR-013), no longer the mock swipe-style matching UI — it's the real browse-open-meetups surface, with a "Schedule a Meetup" entry point into `features/meetups/schedule_flow.dart`. `features/meetups/` holds the whole meetup-scheduling feature: `schedule_flow.dart`, `meetup_detail_page.dart` (includes the Safety Gate sub-flow), `my_meetups_page.dart` (host request management), and `widgets/` for the platform-split location picker (`map_location_step.dart` dispatches to `ios_map_location_step.dart` or `stadia_map_location_step.dart` — see `TESTING-NOTES.md` for why the Android half is still provisional).

`AppShell` ([frontend/lib/app_shell.dart](frontend/lib/app_shell.dart)) is a simple `IndexedStack`-style page switcher driven by local `setState`, not nested routing.

### Trust-level gating

`IntentType` ([frontend/lib/core/models/intent_type.dart](frontend/lib/core/models/intent_type.dart)) encodes both display metadata (label/icon) and a `requiredTrustLevel` per intent — `rideShare`/`dating` require trust level 4, everything else requires **level 2** (raised from 1 by ADR-013 §2 — hosting/joining a real-world meetup with a stranger is the level ADR-006 already calls "the real floor for interacting with strangers," not mere LinkedIn sign-in), exposed via `isUnlockedFor(trustLevel)`. This is mirrored server-side in `backend/services/meetup/internal/service/trustgate.go` — a change to one side must be made on the other, and the server-side check is the one that's actually enforced; the client-side gate is UX only. `UserProfile.trustLevel` drives this gating throughout the meetup/home features — check this enum before adding a new intent type or changing unlock rules.

**Scope gap vs. [ADR-004](docs/04-decisions/adr-004-defer-dating-and-open-ride-sharing.md), now also a Play Store policy question:** `dating` and `rideShare` are already full `IntentType` members and appear in `intent_picker_sheet.dart`'s selectable list (it iterates `IntentType.values`) and in the landing page's `orbiting_intents.dart` (both under `frontend/lib/features/`). ADR-004 defers both to Phase 2/3 — they should exist only as modeled-but-inert enum values for now. Don't build out matching/chat/safety infrastructure behind these two intents as part of Phase 1 work without checking the roadmap first; flag it if a task seems to ask for that. As of the 2026-08-18 App Store/Play Store compliance research (`docs/07-research/app-store-and-play-store-compliance.md`), this is no longer just a scope-discipline concern — Google Play's dating-app age-verification policy applies even to apps where dating is only an incidental, selectable feature, and this app has no age gate anywhere. Don't add real dating functionality behind this intent without that policy question being resolved first (most likely: hide it from the UI entirely until Phase 2 actually builds it).

### Design system

Dark, glassmorphism-style UI. All colors are tokens in `AppPalette` ([frontend/lib/core/theme/app_palette.dart](frontend/lib/core/theme/app_palette.dart)) — never hardcode colors in feature widgets. Shared chrome lives in `frontend/lib/core/widgets/`:
- `Glass` — the base frosted-glass container (`BackdropFilter` blur + tinted border), composed into `GlassTextField`, `GlassBottomBar`.
- `GradientButton`, `AppBackground`, `AppIcon`, `SectionLabel`, `ProfessionalAvatar`, `SkeletonBox` — reused across features instead of ad hoc styling.

### Feature folder layout

`frontend/lib/features/<feature>/` holds the page, with a `widgets/` subfolder for composed pieces used only by that page (e.g. `features/home/widgets/`, `features/landing/widgets/`). Cross-feature reusable pieces belong in `frontend/lib/core/widgets/` instead, not duplicated per-feature.

## Security rule

Per [ADR-003](docs/04-decisions/adr-003-ephemeral-work-email-verification.md), a raw work-email address must never be stored past the verification round-trip — persist only `company_domain`, `work_email_verified`, and `verified_at` (90-day re-verification), never the address itself. This is a live rule now, not forward-looking — `backend/services/auth`'s Postgres schema and the Level 2/3 verification code (committed `b94ad1d`) are what it actually governs; check `verifyAndConsumeCode`/the corporate-email verification path before touching that flow.

## Security review rule

Every non-trivial security review — before marking backend/frontend auth, verification, or payment-adjacent work "reviewed" — walk [Security Review Framework](docs/03-architecture/security-review-framework.md)'s six properties explicitly (Confidentiality, Integrity, Availability, Authenticity, Non-repudiation, Authorization/accountability), not just an unstructured bug hunt. It complements [Threat Model](docs/03-architecture/threat-model.md) (scenario-based) rather than replacing it. As of 2026-08-19 it has one open, unfixed finding worth knowing before touching `internal/identity`: the Apple/Google id_token verification has no `nonce` check, a real (if narrow) replay exposure — see the framework doc's Authenticity section for the fix shape, which mirrors LinkedIn's existing `state`-based CSRF pattern.

## Git rule

Commit docs and the code implementing them together where practical (e.g. a `docs/` change alongside the feature that implements it), per `docs/00-project/cowork-operating-charter.md`.
