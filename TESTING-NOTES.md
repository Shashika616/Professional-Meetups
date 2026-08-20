# Testing-only OTP bypass — DO NOT SHIP TO PRODUCTION

Added so the app can be exercised end-to-end (phone / personal-email /
corporate-email verification) without a real Twilio/Resend send — every OTP
input across the app now accepts the hardcoded code **`123456`** instead of
the code that was actually generated and sent.

This is a single change in one shared backend function — `verifyAndConsumeCode`
(`services/auth/internal/service/verification.go`) calls `otpMatches` for all
three verification purposes (phone, personal email, corporate email), so
one edit covers all of them. Nothing on the frontend needed to change; the
frontend never validates the code itself, it just forwards whatever the user
typed to the backend (per `CLAUDE.md`'s "the client never decides
trust/validity" rule).

## Files changed

- **`backend/services/auth/internal/service/otp.go`**
  - `otpMatches(hash, code string) bool` — the real
    `subtle.ConstantTimeCompare(...)` line is commented out (not deleted)
    and replaced with `return code == "123456"`.
  - The `"crypto/subtle"` import is commented out too (it would otherwise be
    an unused-import build error with the real comparison disabled).

That's the only file touched.

## How to revert before production

In `backend/services/auth/internal/service/otp.go`:

1. Uncomment `"crypto/subtle"` in the `import (...)` block.
2. In `otpMatches`, delete `return code == "123456"` and uncomment
   `return subtle.ConstantTimeCompare([]byte(hash), []byte(hashOTP(code))) == 1`.
3. Delete the `// TESTING BYPASS — DO NOT SHIP TO PRODUCTION ...` doc comment
   above the function (or trim it back to the original one-line comment).
4. Delete this file (`TESTING-NOTES.md`).
5. Rebuild/redeploy the `auth` service (`docker compose up --build auth` in
   `backend/`, or however it's deployed) — this is compiled Go, so the old
   binary keeps accepting `123456` until it's rebuilt.

## Known side effect while this is active

Five backend tests in `backend/services/auth/internal/service` fail while
the bypass is active, because they generate a real OTP and assert that
*that* code (not `123456`) is accepted:

- `TestVerifyPhoneCode_FullRoundTrip`
- `TestVerifyCorporateEmailCode_ExtractsDomainAndDeletesRawAddress`
- `TestVerifyPhoneCode_ConflictWhenAlreadyVerifiedOnDifferentAccount`
- `TestPhoneVerification_Integration`
- `TestPersonalEmailVerification_Integration`
- `TestCorporateEmailVerification_Integration`
- `TestGetProfile_Integration_NeverReturnsRawContactInfo`

This is expected, not a regression — they'll pass again immediately after
the revert above. Don't "fix" them to expect `123456` instead; that would
make the bypass look like a permanent, tested feature rather than the
temporary shortcut it is.

## Rebuilding the running containers

After the code change (already applied), rebuild so the running `auth`
service actually picks it up:

```bash
cd backend
docker compose up --build -d auth
```

# Testing-only map provider: Stadia Maps — not the production decision

Added so the Schedule flow's location step (frontend/meetup-scheduling-
PLAN.md Step 7) can actually be exercised — a real map, search, and "use my
current location" — without waiting on the still-open final vendor choice.
**This is explicitly provisional, not a production decision.** Per
[ADR-013 §4's second correction](docs/04-decisions/adr-013-host-initiated-meetup-scheduling-with-join-requests.md):
neither Mapbox's card requirement nor its Sri Lanka address-search coverage
could be confirmed, so a **Stadia Maps** key (no card required, confirmed)
was created purely to get this step running and testable now. The eventual
production choice remains open between **Google Maps and Mapbox**.

## Files changed

- **`frontend/lib/features/meetups/widgets/map_location_step.dart`** (new)
  — `MapLocationStep`, the real location-step widget: a `maplibre_gl` map
  centered on a fixed crosshair pin, a search bar calling Stadia's
  Geocoding/Autocomplete **v1** endpoint (`api.stadiamaps.com/geocoding/v1/autocomplete`
  — deliberately v1, not v2: v2's autocomplete response is a reduced shape
  with no geometry, requiring a second "place details" call for
  coordinates; v1 returns full GeoJSON with `geometry.coordinates` in one
  round trip, which is all this widget needs), and "use my current
  location" via `geolocator`. **This is the only file that imports
  `maplibre_gl` or talks to Stadia's API** — the tile style URL
  (`tiles.stadiamaps.com/styles/alidade_smooth_dark.json`, confirmed
  directly against docs.stadiamaps.com, not assumed) and the geocoding
  endpoint are both private constants here, not referenced anywhere else.
  Also holds `@visibleForTesting String? debugStadiaApiKeyOverride` — a
  test-only escape hatch (see `frontend/test/schedule_flow_test.dart`)
  that must never be set from real app code.
- **`frontend/lib/features/meetups/schedule_flow.dart`** — the `_Step.location`
  switch case now builds `MapLocationStep` instead of `_LocationStep`. The
  original manual lat/lng/label entry stopgap (`_LocationStep`/
  `_LocationStepState`) is still in the file, fully intact, **commented
  out** — kept as the emergency fallback if the active map provider ever
  needs to be temporarily disabled (a bad/expired key, a provider outage
  during a demo). To bring it back: uncomment that block and swap the
  switch case back to `_LocationStep(...)`.
- **`frontend/lib/core/config/app_config.dart`** — added
  `stadiaMapsApiKey`, same `String.fromEnvironment`/`--dart-define`
  pattern as every other credential in this file, empty default. Pass the
  real value via `--dart-define=STADIA_MAPS_API_KEY=...`; it is never
  written into source. Unlike the Twilio/Resend empty-means-fallback
  pattern, an empty key here means the location step shows a "not
  configured" message and never attempts to render the map.
- **`frontend/pubspec.yaml`** — added `maplibre_gl: ^0.26.2` (renders
  MapLibre-style vector tiles, which Stadia serves — a deliberate
  divergence from ADR-013's original `mapbox_maps_flutter` dependency,
  only for this testing pass; `mapbox_maps_flutter` was **not** also
  added) and `geolocator: ^14.0.2` (pinned below the newest 14.0.3 — its
  transitive `package_info_plus`/`win32` bump conflicts with
  `flutter_secure_storage`'s own `win32` constraint; this app has no
  Windows target, but `pub` still resolves across every platform a
  federated plugin declares).
- **`frontend/ios/Runner/Info.plist`** — added
  `NSLocationWhenInUseUsageDescription` (this is the point the app first
  actually calls `geolocator`, per `CLAUDE.md`'s standing note that this
  key needed to be added before that happened).
- **`frontend/android/app/src/main/AndroidManifest.xml`** — added
  `ACCESS_FINE_LOCATION`/`ACCESS_COARSE_LOCATION`.
- **`frontend/test/schedule_flow_test.dart`** — the capacity-stepper/
  timing-step tests now set `debugStadiaApiKeyOverride` to get past the
  Location step (confirmed empirically that `MapLibreMap` renders safely
  under `flutter_test` with a bogus key/style URL — no real platform view
  or network call happens in that environment); a new test covers the
  real default (no key configured) "not configured" state.

## How this gets superseded

Once the real production vendor is chosen, replacing this is **not** just
an env var change in the general case:

- **If the final choice is Mapbox or Google Maps**: each has its own,
  different, incompatible Flutter SDK (`mapbox_maps_flutter` /
  `google_maps_flutter`). Superseding this means writing a **new widget**
  against that SDK — contained to one file/one `widgets/` addition by this
  design (swap it in for `MapLocationStep` in `schedule_flow.dart`'s
  switch case, same narrow `onSubmit(lat, lng, label)` contract), but real
  implementation work, not a config change. `maplibre_gl` and this file
  can be deleted once that swap is done and verified.
- **If the final choice is another MapLibre-compatible provider** (e.g.
  MapTiler): this really is close to a config-only change — swap the
  style URL and API key in this same file.

Either way: `debugStadiaApiKeyOverride`'s test call sites in
`schedule_flow_test.dart` should be updated or removed alongside whatever
replaces this widget, not left pointing at a symbol that no longer exists.

## Known caveats while this is active

- A real Stadia key must be passed via
  `--dart-define=STADIA_MAPS_API_KEY=...` at `flutter run`/`flutter build`
  time to actually use the map — without it, the location step is a dead
  end (by design: "not configured" rather than a blank/broken map), and
  **the Schedule flow cannot be completed at all** past that point. Copy
  `frontend/.env.example` to `frontend/.env` (gitignored) and fill in the
  key, then use one of the convenience entry points added alongside this
  so you don't have to type the flag by hand: `./frontend/run.sh` (any
  normal `flutter run` args also work, e.g. `./run.sh -d ios`), or the
  "Flutter (.env — iOS/default)" / "Flutter (.env — Android emulator)"
  configs in `.vscode/launch.json`. Plain `flutter run --dart-define-from-file=.env`
  still works directly too — these are just shortcuts, not a new
  mechanism. **`flutter_dotenv` was deliberately not used** — it would
  bundle `.env` into the compiled app as a plaintext asset, extractable
  from any built APK/IPA via `unzip`, which is the wrong tradeoff for a
  real, billable API key.
- The first "use my current location" tap triggers a real OS location
  permission prompt (iOS/Android) — this is the first place in the app
  that does.
- `flutter pub get` is required after this change to pick up the two new
  dependencies.
- Stadia Maps has its own free-tier limits and (like any metered vendor)
  should be reviewed before this goes live more broadly — same discipline
  ADR-013 §4 already flags for whichever provider is chosen for real.

# Platform-split location picker (Apple MapKit on iOS) + the address-search bug fix

Per [ADR-013 §4's third correction](docs/04-decisions/adr-013-host-initiated-meetup-scheduling-with-join-requests.md):
the map-provider question splits by platform now. **iOS is settled** —
Apple MapKit, bundled with the OS, no API key, no billing account, not
provisional. **Android stays on Stadia Maps**, still provisional (the
section above still applies to it). This pass also fixed a real bug in
the address search that shipped with the original Stadia-only widget.

## The search bug — root cause

**None of the four candidates the base plan named were the actual cause.**
The plan's diagnostic order was: (1) is `STADIA_MAPS_API_KEY` actually
populated at run time, (2) does the request reach Stadia's endpoint, (3)
does it return non-200 (wrong param name/auth placement), (4) does
response parsing fail. Checked in order, against the real key already in
`frontend/.env`:

1. Populated — confirmed (the map itself was rendering, which needs the
   same key).
2. Reached — confirmed by replaying the exact request the widget builds
   (same URL, same query params) with `curl` directly against
   `api.stadiamaps.com`.
3. Returned 200 — confirmed, with a real, correctly-shaped GeoJSON
   `FeatureCollection` body.
4. Parsed correctly — confirmed; `properties.label` and
   `geometry.coordinates` were both present exactly as
   `_GeocodeResult.fromFeature` expected.

**The actual bug was a fifth thing: a UI clipping bug downstream of a
fully successful fetch+parse.** `Stack`'s default `clipBehavior` is
`Clip.hardEdge` (not `Clip.none`), and the `Stack` wrapping the search
field only sizes itself to the field's own height (its one non-`Positioned`
child). The suggestions dropdown — `Positioned` below that — was being
silently clipped away every time, regardless of how many results came
back. The network call and the data were never the problem; nothing was
ever visible to tap. Fixing just `clipBehavior: Clip.none` surfaced a
*second* real bug caught by this pass's own widget tests (a `tap()` on a
dropdown item landed on the map's render object instead, per a genuine
hit-test failure): once unclipped, the dropdown could overflow down into
the space the map occupied, and since the search field and the map were
in *separate* `Stack`s, paint order put the map on top wherever they
overlapped. The real fix put the search field and the map inside one
shared outer `Stack` (as a single `Column`), with the dropdown as a later
sibling in that same `Stack` — Positioned children painted last are drawn
on top of everything before them, guaranteeing the dropdown always wins
regardless of how far it overflows. Applied identically to both the Stadia
(Android) and MapKit (iOS) widgets, since both share the same layout
shape.

## Platform split — what changed

- **`frontend/lib/features/meetups/widgets/map_location_step.dart`** —
  no longer the Stadia implementation itself; now a thin platform switch.
  Keeps the name and the `onSubmit(lat, lng, label)` contract
  `schedule_flow.dart` already depends on, and picks between:
  - **iOS**: `IosMapLocationStep` (below).
  - **Everything else (Android, and a reasonable fallback for any other
    target)**: `StadiaMapLocationStep` (below).

  Checked via `defaultTargetPlatform` (`package:flutter/foundation.dart`),
  not `dart:io`'s `Platform.isIOS` — the latter throws on web, which this
  app also builds for (`CLAUDE.md`'s documented `flutter build web`).
- **`frontend/lib/features/meetups/widgets/stadia_map_location_step.dart`**
  (new — this *is* the file the section above calls
  `map_location_step.dart`; renamed when the platform split landed) — the
  Stadia implementation, with the clipping fix, the shared-Stack z-order
  fix, and the new interaction contract from Step 2 below. `debugStadiaApiKeyOverride`
  lives here now, not in `map_location_step.dart` — anything importing it
  needs updating.
  - Also gained an injectable `httpClient` constructor param (same
    optional-override pattern as `HttpMeetupService`/`HttpAuthService`)
    purely for testability — widget tests can now assert on real request
    counts instead of eyeballing debounce behavior.
  - Also gained a locally-tracked `_pickedLocation` field, set directly by
    every path that picks a location (suggestion select, direct search,
    current location) and kept in sync with manual map drags via
    `onCameraIdle`. Previously `_submit()` read `_controller.cameraPosition`
    directly — correct on a real device once the camera animation
    settles, but untestable (`MapLibreMap` never gets a real native
    platform view under `flutter_test`, so `cameraPosition` never
    updates there) and dependent on `animateCamera`'s async timing even on
    a real device. This is a genuine correctness improvement, not just a
    test workaround.
- **`frontend/lib/features/meetups/widgets/ios_map_location_step.dart`**
  (new) — `IosMapLocationStep`: `apple_maps_flutter`'s `AppleMap` for
  rendering, the same fixed-crosshair-pin pattern, the same "Choose a
  public place..." safety copy, `geolocator` for "use my current
  location" (same as Android). **No API key, no `AppConfig` entry, no
  billing setup** — confirmed by running with `STADIA_MAPS_API_KEY` unset
  entirely (this file doesn't import `AppConfig` at all).
- **`frontend/lib/features/meetups/widgets/ios_local_search.dart`** (new)
  — `IosLocalSearch`, a thin Dart wrapper around a native `MethodChannel`
  (`professionalconnections/ios_local_search`). `apple_maps_flutter` only
  renders the map; it doesn't expose MapKit's search APIs, so this
  bridges to them directly rather than silently downgrading iOS to a
  weaker search experience than MapKit natively offers, per the plan's
  own explicit instruction. A raw `MKLocalSearchCompletion` can't cross
  the channel boundary, so `autocomplete()` returns lightweight
  title/subtitle pairs and the native side keeps the last batch;
  `resolveCompletion(index)` asks it to resolve one of those by index —
  mirroring how MapKit itself requires a second `MKLocalSearch` to turn a
  completion into real coordinates, not an extra round trip this wrapper
  invented.
- **`frontend/ios/Runner/LocalSearchChannel.swift`** (new) — the native
  side: `MKLocalSearchCompleter` for debounced completions,
  `MKLocalSearch` for both resolving a completion and the direct-submit
  path. Registered in `frontend/ios/Runner/AppDelegate.swift` via
  `didInitializeImplicitFlutterEngine` (this project's Flutter version
  uses the newer `FlutterImplicitEngineDelegate` embedding API, not the
  classic `application(_:didFinishLaunchingWithOptions:)`-only pattern).
  **Had to be added to `Runner.xcodeproj`'s build target explicitly** —
  a new Swift file on disk isn't enough; Xcode only compiles files
  registered in the project's own file references/Compile Sources build
  phase. Done via the `xcodeproj` Ruby gem (already vendored inside the
  local CocoaPods install) rather than hand-editing `project.pbxproj`.
- **`frontend/lib/core/widgets/glass_text_field.dart`** — added optional
  `textInputAction`/`onFieldSubmitted` params (both default to `null`,
  every existing caller unaffected) for the "type a full query and press
  search/return" path (Step 2, point 3) — needed by both platform
  widgets, shared here rather than duplicated.
- **`frontend/pubspec.yaml`** — added `apple_maps_flutter: ^1.4.0`.
  Checked its maintenance status before committing to it per the plan's
  own instruction (19 months since its last pub.dev release, 34 open
  issues) — used anyway since it's still the standard/most-downloaded
  MapKit binding (27.5k weekly downloads) and no better-maintained
  alternative with equivalent MapKit coverage was found; the search gap
  it has (no `MKLocalSearchCompleter` exposure) is exactly why
  `LocalSearchChannel.swift` exists as a small custom bridge instead of a
  second dependency.
- **Swift Package Manager disabled project-wide** (`flutter: config:
  enable-swift-package-manager: false` in `pubspec.yaml`, from the
  session's own earlier fix) — `apple_maps_flutter` ships a `Package.swift`
  that pulls its native SDK from a GitHub-hosted git URL; with SPM on
  (Flutter's default), Xcode resolves that during every iOS build and
  prompts to sign in to GitHub to raise the anonymous rate limit. Disabled
  so it installs through its CocoaPods podspec instead, like every other
  plugin in this project — confirmed via `pod install` (`Installing
  MapLibre (6.27.0)` / `Installing apple_maps_flutter (0.0.1)` both
  showed up as CocoaPods installs, not SPM resolutions) and a full
  `flutter build ios --simulator --no-codesign`, which succeeded with no
  GitHub prompt.
- **`frontend/test/schedule_flow_test.dart`** — import for
  `debugStadiaApiKeyOverride` updated to `stadia_map_location_step.dart`.
- **`frontend/test/stadia_map_location_step_test.dart`**,
  **`frontend/test/ios_map_location_step_test.dart`**,
  **`frontend/test/map_location_step_test.dart`** (all new) — the tests
  from Step 2's own list: debounce genuinely collapses several keystrokes
  into one request (`MockClient` request-counting on Android, a mocked
  `MethodChannel` call-counting on iOS — neither hits a real network/native
  API); suggestion-select and type-and-submit both correctly lead to
  `onSubmit` with the right coordinates; iOS runs with zero Stadia
  configuration; the platform dispatcher picks the right widget per
  platform.

## Verification performed

Both a real Xcode/CocoaPods toolchain and a booted iOS simulator were
available in this environment, so the iOS half was verified beyond
`flutter analyze`/`flutter test`, not just written and hoped-for:
`flutter build ios --simulator --no-codesign` succeeded (this compiles
`LocalSearchChannel.swift` and links `apple_maps_flutter`), and the built
app was installed and launched on a simulator (`xcrun simctl
install`/`launch`) with no crash — confirmed via a live screenshot and
the process still running afterward. `flutter build apk --debug` was also
run to confirm the Android side still compiles after the restructuring.
Manually exercising the actual search UI on-device (typing, tapping a
suggestion, the search keyboard action) was **not** done in this pass —
that's still worth a real hands-on pass before considering either
platform's search fully verified end-to-end.

## Known caveats while this is active

- `apple_maps_flutter`'s maintenance status is genuinely mixed (see
  above) — if MapKit rendering itself misbehaves on some iOS version,
  that package (not `LocalSearchChannel.swift`, which is this project's
  own code) is the first place to look.
- The interaction design keeps the existing `CONTINUE` button as the
  final "commit this pin position" step on both platforms, rather than
  having a suggestion-tap or direct search immediately call `onSubmit`
  and advance the Schedule flow on its own. Read literally, the base
  plan's test wording ("selecting a suggestion... result[s] in `onSubmit`
  being called") could suggest the latter; this pass interpreted "no
  separate confirm tap needed at that point" (Step 2, point 2) as scoped
  to *filling the search field*, not to skipping the crosshair-drag-to-
  adjust affordance the original testing addendum established. If the
  intent was actually immediate auto-submit-on-select, that's a UX change
  to make deliberately, not a fix to this diagnosis.
- `Runner.xcodeproj`'s new file reference for `LocalSearchChannel.swift`
  was added via a script, not Xcode's GUI — if it's ever moved/renamed on
  disk, Xcode (or another `xcodeproj` script) needs to update the
  project file too; a file that exists on disk but isn't in the project's
  Compile Sources build phase silently doesn't get compiled (exactly the
  error this pass hit and fixed).
