# [ADR-014](adr-014-federated-base-identity-optional-linkedin-age-eligibility.md) — Base Identity (Apple / Google / Email+Password), LinkedIn as the Sole Trust-Granting Step, Age Eligibility

## Status

Accepted (2026-08-19), superseding an earlier same-day draft of this ADR that made Apple/Google the *exclusive* account-creation path with LinkedIn as a deferred link-only upgrade. Revised through direct design discussion into the shape below — the earlier draft's core motivation (a real Apple Guideline 4.8 gap) and research still hold; the mechanism changed.

## Context

Today, LinkedIn OAuth ("Level 1a") is the *exclusive* account-creation mechanism ([ADR-011](adr-011-linkedin-onboarding-slice-design.md)) — there is no way to have an account without it. [App Store & Play Store Compliance](../07-research/app-store-and-play-store-compliance.md) §2 flagged this as a likely Apple Guideline 4.8 gap.

Research (2026-08-19) confirmed the gap is real: **Apple Guideline 4.8** requires an equivalent Sign in with Apple option in any app that offers a third-party/social login (LinkedIn is explicitly named as a trigger service) — not that Apple must be the *only* option, but that it must be offered, equally prominently, alongside whichever others are shown. **Google Play has no equivalent mandate.** Confirmed via research: adding "Sign in with Google" is a UX/parity choice on Android, not a compliance requirement. Adding a plain email+password path doesn't interact with 4.8 at all — Apple's guideline is specifically about third-party/social logins; "your own account system" is the exemption case, not a trigger.

Separately, **Apple's 2025 age-rating overhaul** added a real 18+ band, and starting 2026-02-24 Australia/Brazil/Singapore require 18+ apps to confirm adulthood via "reasonable methods" (self-attestation alone won't be enough there, with signals of broader rollout through 2026). **Google Play's "Restrict Declared Minors"** has been mandatory since 2026-01-28 for apps "facilitating dating," relevant given this app's dormant `dating` IntentType.

## Decision

### Sign-up: three parallel, co-equal entry points

1. **Apple ID (iOS) / Google (Android)** — one-tap federated sign-up via each platform's native SDK. Creates a Level 0 account. **Permanently zero-trust**: linking Apple and/or Google to an account, in any combination, never raises trust level, no matter how many are linked. They prove *device/platform identity*, not *professional identity* — that distinction, not verification strength, is why they're capped. Apple's own OIDC signature verification is already a real, sufficient cryptographic check; nothing further needs adding on top of it.
2. **Email + password** — a real account-system path, offered because not every user wants to federate through Apple/Google (a normal, common pattern on comparable platforms). Signup requires OTP verification of the email as part of the flow, reusing the OTP mechanism already built for Level 2's personal-email verification ([ADR-012](adr-012-level-2-3-verification-delivery-and-identity-anchors.md)) rather than inventing a second one. On success, `personal_email` is populated and marked verified immediately — a head start toward Level 2 later, but **not** toward Level 1: LinkedIn is still required for that, regardless of entry path. No separate "username" field — the OTP-verified email is the login identifier directly; adding a second unique identifier with no real benefit was considered and rejected.
3. **LinkedIn (direct)** — unchanged from today. Grants Level 1 immediately, unique on `linkedin_sub`, exactly as [ADR-011](adr-011-linkedin-onboarding-slice-design.md) already built it.

All three are gated by a single, mandatory, blocking **age-eligibility confirmation** (self-attestation checkbox, no date of birth stored — [ADR-003](adr-003-ephemeral-work-email-verification.md)'s minimal-retention spirit) shown once, before any of the three paths can complete — not after picking one. LinkedIn direct-signup gets this gate added to it too; it doesn't have one today.

### Sign-in: same buttons, one exception

Apple/Google/LinkedIn are **resolve-or-create, not separate signup/signin flows** — the same tap works whether the account already exists or not (see Identity Resolution below), so no dedicated "sign in" screen is needed for the federated paths. Email+password is the one exception: password login inherently needs its own form (a password can't collapse "resolve or create" into one tap the way OAuth does), so it gets a distinct sign-in screen.

### Identity resolution and account linking (the load-bearing new design piece)

This is what makes three parallel entry points safe rather than a source of duplicate or hijacked accounts. Three explicit rules, one shared function per rule — not per-screen logic that could drift:

1. **`resolveOrCreateIdentity(provider, subject, email, name)`** — the single function every federated login (Apple, Google, LinkedIn) calls, regardless of which screen or button triggered it, regardless of whether the user thinks they're "signing up" or "signing in." If `(provider, subject)` already exists, returns that account. If not, creates a new one. There is deliberately no separate signup-handler-vs-signin-handler code path for federated methods — a second implementation of the same lookup is exactly how this kind of bug creeps in.
2. **`linkIdentityToUser(user_id, provider, subject)`** — used when an *already-authenticated* session adds a new identity (the Profile-page "Connect LinkedIn" flow, or optionally "add Apple/Google as backup sign-in" later). If `(provider, subject)` already belongs to a *different* account, this **hard-rejects** with a clear, user-facing error ("this LinkedIn account is already linked to another Professional Meetups account"). Never silently merges two accounts — silent merging is a real account-takeover vector (it would let anyone who can trigger a link claim someone else's account).
3. **Email+password signup as implicit account recovery**: if the email being signed up with matches an existing account's already-*verified* `personal_email`, and the signup's OTP step succeeds (real proof of inbox control), treat this as **recovery**, not a new account and not a hard reject — set a `password_hash` on the *existing* account and log into it. This is deliberately the one place the system resolves across providers based on second-hand evidence, specifically because OTP-verified email ownership is already this system's established trust anchor for that field ([ADR-012](adr-012-level-2-3-verification-delivery-and-identity-anchors.md)). It also solves a real problem otherwise unsolved: someone who loses their Apple device or Google account access would otherwise be locked out permanently.

**What stays explicitly unsolved, on purpose**: the same human signing up once via Apple and separately via Google, with no shared verified identifier between the two (never adds a matching email or LinkedIn to either) — genuinely undetectable, and not worth engineering around. Mitigation is **not prevention**, it's that every account, however many exist, is independently capped by its own trust level — duplicates can't buy functionality, only real verification can, no matter which or how many accounts a person has. This is normal, accepted behavior for this category of app.

### LinkedIn badge

Once LinkedIn is connected (at signup or later via Profile), show a "LinkedIn Verified" badge on the profile — paired with the same non-over-trust caveat [Trust Levels](../02-domain/trust-levels.md)' Design Note already requires for any verification badge ("this account passed a check," never "this person is safe").

## Consequences

- **Schema**: `user_identities (user_id, provider, subject, email, linked_at)` for Apple/Google only (unique on `(provider, subject)` and `(user_id, provider)`), a new nullable `password_hash` on `users`, `age_confirmed_over_18`/`age_confirmed_at` on `users`. `linkedin_sub` stays on `users` exactly as today, whether set at direct signup or later via Profile-linking — not moved into `user_identities`, keeping one consistent home for "does this user have LinkedIn" regardless of when it was connected.
- **`services/auth/internal/linkedin/client.go` and its route are unchanged** — no breaking change to the existing unauthenticated LinkedIn signup path, unlike the earlier draft of this ADR. A new, separate authenticated route handles Profile-initiated linking.
- **New backend surface, sized honestly**: password hashing (argon2id — new dependency decision, not silently picked), a login endpoint, extending the existing Redis rate limiter to cover login attempts by identifier (not just IP+path, which is all it does today), and a password-reset flow that reuses the existing OTP-via-email mechanism rather than building a second one. Smaller than a from-scratch password system would be, precisely because it leans on infrastructure [ADR-012](adr-012-level-2-3-verification-delivery-and-identity-anchors.md) already built.
- **`computeTrustLevel`** needs a Level-0 branch (`linkedin_sub == "" → 0`) — today it hardcodes the opposite assumption in its own comment. Same audit requirement as before: anywhere that treated "authenticated" as synonymous with "at least Level 1" needs checking.
- Resolves the real Apple 4.8 gap: Apple is offered, equally prominent alongside Google/LinkedIn wherever those appear on iOS.
- **Known, accepted limitation, not solved here**: self-attestation age confirmation doesn't meet Apple's stricter regional "reasonable methods" bar (AU/BR/SG today, expanding) or Google's dating-app age-verification exemption bar. Tracked as an open gap in [App Store & Play Store Compliance](../07-research/app-store-and-play-store-compliance.md), not treated as closed.

## Open questions (recommendation given, final call is Shashika's)

- Should Profile also offer "add Apple/Google as a backup sign-in method" and "add a password" for accounts that started elsewhere? Not required for v1 — flagging as a natural follow-on now that the linking primitives exist, not committing to build it in this slice.
- Level 1b (pasted-URL LinkedIn fallback) — still recommended for retirement, unchanged from the earlier draft: it existed only to unblock account creation when OAuth wasn't available, and now three other paths already do that.

## Draft copy (starting point, not final — needs the legal/PDPA pass already tracked in Action Tracker)

Age/safety screen, shown once, before any sign-up path, blocking:

> **You must be 18 or older to use Professional Meetups.**
> ☐ I confirm I am 18 years of age or older.
>
> We verify professional identity and give you tools to plan safer in-person meetups — but only you can judge a situation in the moment. Please use your own judgment, meet in public places, and let someone know where you're going.

Entry screen, below the three buttons:

> Signing in without LinkedIn keeps your account read-only. Connect LinkedIn anytime — during setup or later from your profile — to unlock matching, messaging, and meetups.

## Related

[Trust Levels](../02-domain/trust-levels.md) · [Domain Model](../02-domain/domain-model.md) · [App Store & Play Store Compliance](../07-research/app-store-and-play-store-compliance.md) · [ADR-001](adr-001-verified-real-world-professional-networking.md) · [ADR-003](adr-003-ephemeral-work-email-verification.md) · [ADR-006](adr-006-progressive-linkedin-first-trust-onboarding.md) · [ADR-011](adr-011-linkedin-onboarding-slice-design.md) · [ADR-012](adr-012-level-2-3-verification-delivery-and-identity-anchors.md) · [ADR-013](adr-013-host-initiated-meetup-scheduling-with-join-requests.md)
