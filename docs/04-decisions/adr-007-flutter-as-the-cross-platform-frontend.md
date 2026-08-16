# [ADR-007](adr-007-flutter-as-the-cross-platform-frontend.md): Flutter as the Cross-Platform Frontend (Android + iOS, Web as a Secondary Target)

**Status:** Accepted — partially resolves the "Tech stack" open item in [Project State](../00-project/project-state.md) (frontend only; backend/database/cloud remain undecided).

## Context

[System Architecture](../03-architecture/system-architecture.md) was deliberately written stack-agnostic while the frontend technology was undecided. The mock frontend scaffold (`Professional-Meetups` repo) was built in Flutter targeting Android and web. On 2026-08-16, iOS platform support was added locally via Xcode/`flutter create` scaffolding, confirming the product's real requirement: build once, ship to both Android and iOS, matching [Requirements](../01-product/requirements.md)'s target audience (Sri Lankan professionals, who are split across both mobile OSes) rather than picking one platform first.

## Decision

Flutter is the frontend framework, targeting Android and iOS as the two primary mobile platforms, with web as a secondary/marketing-site-style target. A `macos` platform folder also exists in the repo as an incidental side effect of the scaffolding tool — it is not an active target and shouldn't be treated as a product decision to support desktop macOS.

## Consequences

- **Cross-platform permission handling becomes a real, near-term concern.** Several safety-critical features in [Safety Features Catalog](../03-architecture/safety-features-catalog.md) (live location sharing, SOS, meeting check-in) and [Verification Model](../02-domain/verification-model.md) § 7 (KYC/liveness selfie) require OS-level permissions that must be declared on *both* platforms independently: iOS usage-description keys in `ios/Runner/Info.plist` (e.g. `NSLocationWhenInUseUsageDescription`, `NSCameraUsageDescription`) and the equivalent Android manifest permissions. Neither exists yet — the app has no plugins requiring them yet — but this needs to happen before, not during, the location/SOS/KYC build-out, since a missing iOS usage-description string causes an instant crash on first permission request, not a graceful failure.
- **App Store review adds a second compliance surface beyond Google Play.** Apple's App Review has its own scrutiny of apps that facilitate real-world meetups between strangers (comparable to how dating apps are reviewed) — the safety-gate, reporting, and blocking features in [Safety Features Catalog](../03-architecture/safety-features-catalog.md) aren't just good practice, they're likely necessary to pass review. Worth a dedicated pre-submission check against current App Store guidelines once the app is feature-complete enough to submit.
- **CI now needs a macOS runner for any iOS-specific pipeline steps** (e.g. `flutter build ios`, CocoaPods install) if that's ever added to `.github/workflows/flutter-ci.yml` — the current CI job only runs `flutter analyze` / `flutter test` on `ubuntu-latest`, which doesn't require a macOS runner and stays as-is for now.
- No change to backend architecture — this decision is frontend-only. [System Architecture](../03-architecture/system-architecture.md) remains structure-only for the backend/database/cloud layer.

## Related

[System Architecture](../03-architecture/system-architecture.md) · [Requirements](../01-product/requirements.md) · [Safety Features Catalog](../03-architecture/safety-features-catalog.md) · [Verification Model](../02-domain/verification-model.md) · [Project State](../00-project/project-state.md)
