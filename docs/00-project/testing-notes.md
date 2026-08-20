# Testing Notes

**Pointer note, not the full detail** — the actual, code-accurate record of every active testing shortcut lives in `TESTING-NOTES.md` at the repo root (kept there, not in `docs/`, because it's a live working note Claude Code edits directly as shortcuts are added/removed, not generated from this vault). This vault note exists so anyone scanning product/architecture docs first — the normal entry point per `CLAUDE.md` — doesn't miss that shortcuts are active. **Read `TESTING-NOTES.md` before assuming any of the following is real, tested behavior.**

**Anyone reading this should ask**: is what I'm looking at in the running app real, or one of these active shortcuts?

## Active shortcuts, as of 2026-08-18

1. **OTP hardcoded bypass.** Every phone/personal-email/corporate-email verification code input accepts `123456` instead of the real code that was generated and sent — added so Level 2/3 trust-level flows can be exercised end-to-end without live Twilio/Resend sends. Single change, in `backend/services/auth/internal/service/otp.go`'s `otpMatches` — the real `subtle.ConstantTimeCompare` is commented out, not deleted. **Never ship with this active** — it means anyone who knows to type `123456` can pass phone/personal-email/corporate-email verification for any account. Five backend tests fail while it's active, by design (they assert the *real* code is required) — that's the tripwire that should catch this if a build is ever attempted with it still in place.
2. **Stadia Maps as the Android map/location-picker provider.** Not the final Android decision — see [ADR-013](../04-decisions/adr-013-host-initiated-meetup-scheduling-with-join-requests.md) §4's second and third corrections. Shashika created a Stadia Maps API key (no credit card required) purely to get the Schedule-a-meetup location step actually running and testable, after Mapbox's card requirement and its Sri Lanka address-search coverage couldn't be confirmed with certainty. The final Android choice is still open between Stadia, Google Maps, and Mapbox. **iOS is no longer part of this open question** (third correction, 2026-08-18) — iOS uses Apple's native MapKit, settled, not provisional: free, no API key, no billing account, ships with the OS. Only the Android half is still a "temporary, revisit later" shortcut. Built so the Android swap is contained (existing manual-entry stopgap commented out, not deleted; Stadia's widget sits behind the same narrow contract) — but swapping Android to Mapbox or Google specifically means a new widget against a different Flutter package (MapLibre vs. their own SDKs), not a config-only change. Low risk if forgotten (worst case: the app just keeps working against Stadia Maps, which is a real, functioning provider — unlike the OTP bypass, this isn't a safety hole), but still worth resolving deliberately rather than letting a "temporary" choice become the permanent one by default.

## How to check current status

`git log -- TESTING-NOTES.md` and the file's own content are the actual source of truth for what's active right now — this vault note is a snapshot as of the date above and can drift. Re-read `TESTING-NOTES.md` directly before any production-readiness conversation.

## Related

[Action Tracker](action-tracker.md) · [ADR-013](../04-decisions/adr-013-host-initiated-meetup-scheduling-with-join-requests.md) · `TESTING-NOTES.md` (repo root)
