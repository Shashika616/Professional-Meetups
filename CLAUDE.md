# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Documentation

Business/product truth for this app lives in `docs/` (exported from the Cowork product/architecture vault, not written by hand — see `docs/00-project/cowork-operating-charter.md` for how the two tools divide labor). Read the relevant doc before making a significant architecture or business-rule change; don't re-derive requirements from the UI code.

- `docs/00-project/vision.md`, `docs/00-project/project-state.md` — what we're building, and current status/open questions.
- `docs/01-product/requirements.md`, `docs/01-product/personas.md` — functional/non-functional requirements, target users.
- `docs/02-domain/domain-model.md`, `docs/02-domain/trust-levels.md`, `docs/02-domain/verification-model.md` — the domain vocabulary and the trust-level/verification model the app implements.
- `docs/03-architecture/` — system architecture, threat model, trust & safety architecture, safety features catalog, privacy/anti-abuse controls, operations & incident response.
- `docs/04-decisions/` — ADR-001 through ADR-007. Why decisions were made; don't silently re-decide something recorded here — if a decision looks wrong once you're in the code, say so and propose a new ADR rather than implementing around it.
- `docs/05-ux/safety-ux-flows.md`, `docs/06-roadmap/roadmap.md`, `docs/07-research/` — UX flows, phased roadmap, legal/market research backing the above.

`docs/` is generated — never hand-edit files under it. The vault ("Professional Meetups Vault" in the user's Documents folder) is the source of truth; after editing a note there, run `python3 scripts/sync_docs_from_vault.py` from the repo root to refresh `docs/` (mechanical wikilink-to-relative-link conversion, no content changes — safe and cheap to run anytime).

## Known Gaps (flagged 2026-08-16, not yet fixed)

- **`lib/features/onboarding/onboarding_flow.dart` implements the pre-ADR-006 trust model.** It runs phone → LinkedIn (URL-paste only) → corporate email as three sequential *mandatory* steps before entering `AppShell`, and labels corporate email as unlocking "Level 2 Trust." The current design (`docs/02-domain/trust-levels.md`, `docs/04-decisions/adr-006-progressive-linkedin-first-trust-onboarding.md`) instead wants: LinkedIn (federated OAuth preferred, pasted-URL as a lower-trust fallback that can't unlock matching) as the entry point → phone + personal email + personal details (name/address) as Level 2, the real floor for matching → corporate email as an optional Level 3 booster → optional KYC/liveness as Level 4. `IntentType.requiredTrustLevel` and `UserProfile` also only model levels 1 and 4, with no level 2/3 distinction and no federated-vs-claimed LinkedIn field. **This is intentionally not fixed yet** — flagged here so it isn't rebuilt on top of, until it's explicitly picked up as a task.

## Commands

All commands run from the `Professional-Meetups/` directory (the Flutter project root — note the repo root one level up is just a wrapper folder).

```bash
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
- Open `ios/Runner.xcworkspace` (not `.xcodeproj`) in Xcode for signing/capabilities — CocoaPods wires the Flutter framework into the workspace, not the bare project.
- `ios/Runner/Info.plist` has no usage-description keys yet (no location/camera/photo-library permissions requested) because no plugin needs them yet. Before implementing anything that touches live location, SOS, or KYC selfie capture (`docs/03-architecture/safety-features-catalog.md`, `docs/02-domain/verification-model.md` § 7), add the corresponding `NSLocationWhenInUseUsageDescription` / `NSCameraUsageDescription` etc. here, and the matching Android manifest permissions — both platforms need this before those features can run at all, not just before they can ship.
- `analysis_options.yaml` now excludes `build/**`, `android/**`, `ios/**`, `macos/**`, `web/**`, `windows/**`, `linux/**` from the analyzer — generated platform code won't show up in `flutter analyze` or the CI lint step.

`.github/workflows/flutter-ci.yml` runs on every push/PR to `main`: `dart format --set-exit-if-changed`, `flutter analyze --fatal-infos`, then `flutter test --coverage`. Run all three locally before pushing.

## Architecture

This is a **frontend-only Flutter scaffold** (single "Initial Frontend" commit) for a professional-networking/meetup app. There is no real backend: every service is a `Mock*` implementation that simulates network latency and validation. The codebase is deliberately structured so a real backend can be swapped in later without touching UI code.

### State management: Riverpod

All app-wide state and dependency wiring lives in [lib/core/providers/app_providers.dart](lib/core/providers/app_providers.dart). Services are exposed as plain `Provider`s (e.g. `authServiceProvider`, `matchingServiceProvider`), currently bound to their `Mock*` implementations — this is the single seam to swap in real implementations. UI reads state via `ConsumerWidget`/`ConsumerStatefulWidget` and `ref.watch`/`ref.read`; there's no other state management approach in the app.

### Service-contract pattern

Business logic is defined as `abstract interface class` contracts in `lib/core/services/` (`AuthService`, `MatchingService`), each with a `Mock*` implementation. Doc comments on these interfaces state the operating principle explicitly: **the client never decides trust/validity — it only displays what the server would return**. Mock implementations intentionally throw `FormatException` to simulate server-side rejection. When adding new backend-touching features, follow this same pattern: define the interface, mock it, wire it through `app_providers.dart`.

### Validation

`lib/core/validation/validators.dart` holds pure, static validation functions (phone, OTP, LinkedIn URL, corporate email incl. free-provider/role-based-mailbox rejection). These exist for UX speed only — every mock service re-checks the same rules to simulate the server being the actual source of truth. Keep this split when extending: pure validators for instant UI feedback, service-layer checks for the "server" decision.

### Navigation

No router package — plain `Navigator.push`/`pushReplacement` with `MaterialPageRoute`. Fixed linear flow:

```
main.dart → SplashScreen → LandingPage → OnboardingFlow (4 steps: welcome → phone/OTP → LinkedIn → corporate email)
          → AppShell (bottom nav: Home, Matches, Safety, Chats, Profile)
```

`AppShell` ([lib/app_shell.dart](lib/app_shell.dart)) is a simple `IndexedStack`-style page switcher driven by local `setState`, not nested routing.

### Trust-level gating

`IntentType` ([lib/core/models/intent_type.dart](lib/core/models/intent_type.dart)) encodes both display metadata (label/icon) and a `requiredTrustLevel` per intent (e.g. `rideShare`/`dating` require trust level 4, everything else level 1), exposed via `isUnlockedFor(trustLevel)`. `UserProfile.trustLevel` is meant to drive this gating throughout the matching/home features — check this enum before adding a new intent type or changing unlock rules.

**Scope gap vs. [ADR-004](docs/04-decisions/adr-004-defer-dating-and-open-ride-sharing.md):** `dating` and `rideShare` are already full `IntentType` members and appear in `intent_picker_sheet.dart`'s selectable list (it iterates `IntentType.values`) and in the landing page's `orbiting_intents.dart`. ADR-004 defers both to Phase 2/3 — they should exist only as modeled-but-inert enum values for now. Don't build out matching/chat/safety infrastructure behind these two intents as part of Phase 1 work without checking the roadmap first; flag it if a task seems to ask for that.

### Design system

Dark, glassmorphism-style UI. All colors are tokens in `AppPalette` ([lib/core/theme/app_palette.dart](lib/core/theme/app_palette.dart)) — never hardcode colors in feature widgets. Shared chrome lives in `lib/core/widgets/`:
- `Glass` — the base frosted-glass container (`BackdropFilter` blur + tinted border), composed into `GlassTextField`, `GlassBottomBar`.
- `GradientButton`, `AppBackground`, `AppIcon`, `SectionLabel`, `ProfessionalAvatar`, `SkeletonBox` — reused across features instead of ad hoc styling.

### Feature folder layout

`lib/features/<feature>/` holds the page, with a `widgets/` subfolder for composed pieces used only by that page (e.g. `features/home/widgets/`, `features/landing/widgets/`). Cross-feature reusable pieces belong in `lib/core/widgets/` instead, not duplicated per-feature.

## Security rule (forward-looking)

There's no persistence layer yet, but when one is added: per [ADR-003](docs/04-decisions/adr-003-ephemeral-work-email-verification.md), a raw work-email address must never be stored past the verification round-trip — persist only `company_domain`, `work_email_verified`, and `verified_at` (90-day re-verification), never the address itself.

## Git rule

Commit docs and the code implementing them together where practical (e.g. a `docs/` change alongside the feature that implements it), per `docs/00-project/cowork-operating-charter.md`.
