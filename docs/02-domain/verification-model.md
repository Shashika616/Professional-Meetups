# Verification Model

**Core principle: do not trust any single verification method.** Each method has a known weakness, so the platform layers them plus continuous trust scoring plus safety controls — never relies on one signal alone.

| Method | Weakness |
|---|---|
| Phone number | Disposable SIMs, stolen SIMs, VoIP numbers, SMS OTP interception |
| LinkedIn — federated (OAuth) | Fake profiles, stolen accounts, newly created accounts, purchased aged accounts |
| LinkedIn — claimed (pasted URL) | Not a verification at all; anyone can paste anyone's public profile URL |
| Personal email | Disposable/burner addresses |
| Personal details (name, address) | Self-reported; can be false unless cross-checked or KYC'd |
| Company email | Fake/lookalike company domains, compromised employee email |
| Profile photo | Can be stolen or AI-generated |

## 1. Phone verification

OTP with rate limiting; block high-risk VoIP numbers; detect disposable-number providers; limit accounts per phone number; detect recent SIM changes where possible; prefer device-bound verification; app attestation against bulk OTP bots; no unlimited OTP resends. Where available, prefer carrier-based/silent network verification over SMS OTP alone. Never reveal a user's full phone number to other users.

## 2. LinkedIn verification — the entry point (Level 1)

Two paths, per [ADR-006](../04-decisions/adr-006-progressive-linkedin-first-trust-onboarding.md), not treated as equally trustworthy:

- **Federated (preferred)**: official LinkedIn OAuth/OpenID login. The platform confirms the user actually controls the account. This is Level 1a.
- **Claimed (fallback)**: user pastes their public LinkedIn profile URL instead. **This is not a verification — anyone can paste anyone's public URL.** Product rule (see [Trust Levels](trust-levels.md)): a claimed-only account cannot match or be matched. It exists so onboarding doesn't hard-fail when a user can't or won't do OAuth, but it must not be presented to the user as equivalent to a federated connection, and the UI should visibly label it "unverified" until upgraded.

For federated accounts, cross-check consistency across LinkedIn name, app name, phone-number-owner name if available, email name, profile photo, workplace, and location; flag mismatches. Risk signals: newly created account, very few connections, no profile photo, generic job titles, suspicious employment history, reverse-searched/AI-generated photo, claims of working at a famous company with no evidence. **Limitation: LinkedIn itself can contain fake profiles — treat it as one signal, not proof, even when federated.**

## 3. Personal email verification (Level 2)

A standard OTP/magic-link verification against the user's personal email address (this is separate from, and in addition to, professional/work email — see § 5). Confirms the user controls a real, persistent email address and provides an account-recovery channel independent of LinkedIn or phone.

## 4. Personal details (Level 2)

Legal name and address, captured during the Level 2 upgrade flow. Used for: matching the name against LinkedIn/phone-owner name for consistency checks (§ 2), enabling KYC document matching later if the user reaches Level 4, and — where legally required — identity records for incident response ([Operations & Incident Response](../03-architecture/operations-and-incident-response.md)). Treat address as sensitive: never shown to other users, fuzzed the same way location is (see [Privacy & Anti-Abuse Controls](../03-architecture/privacy-and-anti-abuse-controls.md) § Location privacy).

## 5. Professional (work) email verification — optional trust booster (Level 3)

Not required to use the app (per [ADR-006](../04-decisions/adr-006-progressive-linkedin-first-trust-onboarding.md), this is no longer a mandatory onboarding pillar) — but the strongest available signal short of KYC, and the one with the most legal nuance. See [Sri Lanka PDPA - Work Email Verification](../07-research/sri-lanka-pdpa-work-email-verification.md) and [ADR-003](../04-decisions/adr-003-ephemeral-work-email-verification.md).

- **Reject free email domains** (gmail.com, yahoo.com, hotmail.com, outlook.com, protonmail.com, zoho.com, icloud.com) as professional proof — usable for recovery/communication only.
- **Reject role-based addresses** (info@, admin@, hr@, contact@, support@, careers@, jobs@, office@) — too easy to abuse. Require personal work addresses (firstname.lastname@company.com).
- **Domain verification**: check domain age, DNS/MX records, SPF/DKIM/DMARC, website presence, company LinkedIn page, registration info, public business footprint, employee count, similarity to known brands, recent-registration flags, lookalike patterns (e.g. `examplebank.com` vs. `examplebank-careers.com`).
- **Company verification database**: pre-verify known companies (banks, telecoms, software companies, hospitals, universities, law firms, audit firms, government-linked institutions, large private companies, registered Sri Lankan startups) — see [Domain Model](domain-model.md) § Company. Known domain → trust increases; unknown domain → additional checks required.
- **Unknown company flow**: user enters company details → system checks domain reputation → user may need proof of employment (employee ID, business card, redacted pay slip, employer letter, or corporate-email + LinkedIn consistency) → manual review team approves/rejects. Manual review is acceptable for MVP.
- **Data handling**: verify via OTP/link, extract/confirm the company domain, then **delete the raw email address**. Store only `company_domain`, `work_email_verified = true`, `verified_at`. Re-verify every 90 days; on expiry, show "verification expired," not "unverified" (the person's employment status may not actually have changed). Full legal rationale in [Sri Lanka PDPA - Work Email Verification](../07-research/sri-lanka-pdpa-work-email-verification.md).

## 6. Device-level fraud prevention

Check device fingerprint, app integrity, emulator use, rooted/jailbroken devices, multiple accounts per device, VPN/proxy use, suspicious IP geography, rapid account creation, automated behavior, bulk messaging, repeated OTP attempts, copy-paste profile text, duplicate photos across accounts, one device used across multiple phone numbers. Use Play Integrity API / App Attest / DeviceCheck / Firebase App Check / bot-detection services / invisible CAPTCHA where necessary. **Do not auto-block VPN users** — legitimate professionals use VPNs; raise risk score and require more verification instead.

## 7. KYC / biometric liveness — optional, Level 4

Not built for MVP; introduce if/when the platform needs a stronger "this is a real, live human" guarantee — most likely as a prerequisite for dating or open ride-sharing ([ADR-004](../04-decisions/adr-004-defer-dating-and-open-ride-sharing.md)). Live selfie / face-liveness check against the profile photo, optional government ID upload and matching against the personal details from § 4, optional video verification, optional address proof, background check where legally allowed. Treat all of this as sensitive-document handling per [Privacy & Anti-Abuse Controls](../03-architecture/privacy-and-anti-abuse-controls.md) § Do Not Store Unnecessary Sensitive Documents — collect only what's necessary, encrypt, restrict access, delete after verification where possible.

## Related

[Trust Levels](trust-levels.md) · [Domain Model](domain-model.md) · [Trust & Safety Architecture](../03-architecture/trust-and-safety-architecture.md) · [Sri Lanka PDPA - Work Email Verification](../07-research/sri-lanka-pdpa-work-email-verification.md) · [ADR-003](../04-decisions/adr-003-ephemeral-work-email-verification.md) · [ADR-006](../04-decisions/adr-006-progressive-linkedin-first-trust-onboarding.md) in `04 - Decisions`
