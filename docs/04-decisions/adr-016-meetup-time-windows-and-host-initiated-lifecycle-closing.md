# [ADR-016](adr-016-meetup-time-windows-and-host-initiated-lifecycle-closing.md) — Meetup Time Windows and Host-Initiated Lifecycle Closing

## Status

Accepted (2026-08-20). Corrects [ADR-013](adr-013-host-initiated-meetup-scheduling-with-join-requests.md) (time model) and [ADR-015](adr-015-post-meetup-star-ratings.md) (explicitly declined to revive `completed` — that position is now superseded, see below).

## Context

Two real, user-reported gaps in the built meetup-scheduling slice:

1. **No time entry for "today" meetups, and no time shown on any card.** [ADR-013](adr-013-host-initiated-meetup-scheduling-with-join-requests.md)'s schema made `scheduled_for` nullable, with NULL meaning "now/today" — but that means a host scheduling for today never enters a time at all, and nothing on `MeetupResponse` ever surfaces a time either, even for the "schedule for later" path where `scheduled_for` *is* set. A single point-in-time was also never quite right for a casual meetup anyway — "coffee sometime between 3 and 5" is a more honest signal than an exact instant.
2. **No way to close a meetup, and no open/history distinction.** [ADR-015](adr-015-post-meetup-star-ratings.md) found `meetups.status`'s `completed` value dead and deliberately left it that way, reasoning that rating-eligibility only needed the per-participant `meetup_feedback.happened` trigger, not a global "meetup closed" flag. That reasoning was locally correct but incomplete — it only considered *rating eligibility*, not the separate, real need for a host to mark a meetup done and for "My Meetups" to distinguish current/open meetups from history. Those are a genuine gap this ADR closes; [ADR-015](adr-015-post-meetup-star-ratings.md)'s own scope (rating eligibility) is unaffected and unchanged.

## Decision

### Time windows replace a single instant

`meetups.scheduled_for` (nullable `TIMESTAMPTZ`) is replaced by `window_start`/`window_end` (both `TIMESTAMPTZ NOT NULL`, `CHECK (window_end > window_start)`). Every meetup — "today" or "schedule for later" — now requires a real time range, collapsing what used to be two special-cased paths (NULL-means-now vs. a set instant) into one consistent model. "Today" just means a window whose date happens to be today; no more silent no-time-entered case. Displayed on every meetup card as a range ("Today, 3:00–5:00 PM" / "Aug 22, 6:00–8:00 PM"), not just stored.

### Host-initiated closing, and a real open/closed lifecycle

Revives `meetup_status`'s existing `completed` value (already in the enum since [ADR-013](adr-013-host-initiated-meetup-scheduling-with-join-requests.md), unused until now). A new host-only action, "Close Meetup," transitions `status` → `completed` and stamps a new `closed_at TIMESTAMPTZ` column (mirrors the existing `cancelled_at` pattern). Guardrail: closing is only allowed once `now() >= window_start` — a host can't accidentally close a meetup that hasn't started yet, but isn't forced to wait for `window_end` either, since real meetups run long or short. "My Meetups" gets two views — **Open** (`status IN ('open','full')`) and **History** (`status IN ('completed','cancelled')`).

**Explicitly does not change rating eligibility or the feedback trigger** — those stay exactly as [ADR-015](adr-015-post-meetup-star-ratings.md) defined them: gated on the *individual participant's* `meetup_feedback.happened = true`, completely independent of the meetup's overall open/closed state. Closing is an organizational/display concept (moves a meetup to history); it is not a new security gate layered on top of ratings. Don't couple them.

### Feedback becomes two steps, with an optional note

"How did it go?" (happened / didn't happen — unchanged) is followed by a second, optional step: a free-text note. `meetup_feedback` gains a nullable `notes TEXT` column. The *timestamp* this was asking for — "track them with actual time when confirming, regardless of the scheduled time" — already exists: `meetup_feedback.submitted_at TIMESTAMPTZ NOT NULL DEFAULT now()` has been there since [ADR-013](adr-013-host-initiated-meetup-scheduling-with-join-requests.md)'s original migration and already records real confirmation wall-clock time, independent of `window_start`/`window_end`. Nothing new needed there — worth saying plainly so it isn't rebuilt.

### Rating confirmation

A confirmation step before `SubmitRating` fires — ratings are immutable and one-shot ([ADR-015](adr-015-post-meetup-star-ratings.md)), so confirming before submission (not after) is the right place for a "you can't undo this" moment, rather than relying on the user not fat-fingering a star value.

### Shared-widget bug, unrelated to the above but found via the reported screenshots

`core/widgets/gradient_button.dart`'s label `Text` has no `Flexible`/overflow handling inside its `Row`, and the button's `Container` has a fixed 24px-per-side padding — so a bold, letter-spaced label (e.g. "IT HAPPENED") can exceed its available width when two `GradientButton`s sit side-by-side on a narrower device, producing a visible `RenderFlex` overflow banner. Fixed at the shared-widget level (wrap the label in `Flexible` with `TextOverflow.ellipsis` as a backstop), not patched at the one call site that happened to surface it — the same latent bug exists anywhere else two of these buttons sit in a row with a long label.

## Addendum (2026-08-20): host controls were stranded on the wrong screen

Found using the actual built app: `MyMeetupsPage`'s "Hosting" tab pushes a separate, narrower `_RequestManagementPage` (Accept/Reject only) instead of `MeetupDetailPage`, where `CLOSE MEETUP` actually lives — so the normal way a host reaches their meetups (calendar icon → Hosting) is exactly the one path that can't close or cancel anything. Worse with scale (ten hosted meetups, not one). Also found: `cancelMeetup` already exists end-to-end in the service layer and backend, with no UI anywhere calling it. Fix: extract the host actions (Cancel — new, destructive-styled, confirmed — and Close — now also confirmed, was a direct call before) into one shared widget used by both screens, and add an explicit `MeetupStatusBadge` (Open/Full/Cancelled/Completed) to every card, not just implied by other text. Full detail in `frontend/meetup-lifecycle-PLAN.md`'s addendum.

## Consequences

- Schema: migration `0006` — `meetups` drops `scheduled_for`, adds `window_start`/`window_end` (`NOT NULL`) and `closed_at`; `meetup_feedback` adds `notes`.
- `CreateMeetupRequest`/`MeetupResponse` (`meetup.proto`) replace `scheduled_for_unix_seconds` with `window_start_unix_seconds`/`window_end_unix_seconds`; `MeetupResponse` gains `closed_at_unix_seconds`. `SubmitMeetupFeedbackRequest` gains `optional string notes`.
- New `CloseMeetup` RPC, host-only (ownership + `now() >= window_start` both checked server-side, never client-trusted — same discipline as every other meetup RPC).
- Frontend: schedule flow gets a time-range picker for both "today" and "later" paths (one flow, not two anymore); cards display the range; "My Meetups" gets Open/History tabs; host detail view gets a "Close Meetup" action; feedback flow gets the second optional-note step; rating submission gets a confirm dialog; `GradientButton` gets the overflow fix.

## Related

[Domain Model](../02-domain/domain-model.md) · [ADR-013](adr-013-host-initiated-meetup-scheduling-with-join-requests.md) · [ADR-015](adr-015-post-meetup-star-ratings.md)
