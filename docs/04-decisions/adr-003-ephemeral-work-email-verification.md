# [ADR-003](adr-003-ephemeral-work-email-verification.md): Work-Email Verification Is Real-Time Only — No Persistent Storage of the Raw Address

**Status:** Accepted, with one open item flagged for legal review before launch (see below)

## Context

Professional email is the strongest identity signal available ([Verification Model](../02-domain/verification-model.md) § 3), but storing raw work email addresses long-term creates unnecessary breach exposure and legal risk. Sri Lanka's Personal Data Protection Act (PDPA, No. 9 of 2022) requires processing to have a specified, legitimate purpose, be proportionate, and requires data to be retained only as long as necessary. Full analysis in [Sri Lanka PDPA - Work Email Verification](../07-research/sri-lanka-pdpa-work-email-verification.md).

## Decision

Flow: user enters `john@company.com` → send OTP/verification link → user verifies → extract/confirm `company_domain` → **delete `john@company.com`** → store only `company_domain`, `work_email_verified = true`, `verified_at`. Require re-verification every 90 days; on expiry, surface state as "verification expired," not "unverified" — the person's professional status hasn't necessarily changed, only the verification currency has. Never publicly claim "✓ Verified by Company XYZ" (implies employer endorsement); instead show "Professional Email Verified — Company XYZ" with an on-tap disclaimer that this doesn't indicate employer endorsement.

## Consequences

- Substantially reduces breach exposure and simplifies PDPA compliance (data minimization, accuracy-over-time via re-verification).
- Requires UX for the "expired, please re-verify" state, and requires the onboarding copy to disclose exactly what's checked and what's discarded (see the sample copy in [Sri Lanka PDPA - Work Email Verification](../07-research/sri-lanka-pdpa-work-email-verification.md)).
- Requires the same operational discipline for any other short-lived verification material (ID photos, pay slips, vehicle documents) — see [Privacy & Anti-Abuse Controls](../03-architecture/privacy-and-anti-abuse-controls.md) § Do Not Store Unnecessary Sensitive Documents.
- **Open item — needs a Sri Lankan privacy lawyer before launch**: making work-email verification a *mandatory condition of using the service at all* raises a consent-conditionality question under PDPA (consent isn't "freely given" if access is conditioned on processing not strictly necessary for the service). There's a reasonable argument that verification is fundamental to this specific product (a verified-professionals-only community), but the legal basis and privacy notice should be structured properly rather than relying on a bare "I consent" checkbox. Tracked in [Project State](../00-project/project-state.md) § Blocked.

## Related

[Verification Model](../02-domain/verification-model.md) · [Sri Lanka PDPA - Work Email Verification](../07-research/sri-lanka-pdpa-work-email-verification.md) · [Privacy & Anti-Abuse Controls](../03-architecture/privacy-and-anti-abuse-controls.md) · [Project State](../00-project/project-state.md)
