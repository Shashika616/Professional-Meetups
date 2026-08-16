# Sri Lanka PDPA — Work Email Verification Legal Analysis

Source material behind [ADR-003](../04-decisions/adr-003-ephemeral-work-email-verification.md). Question asked: is it legally sound to require a corporate email address for verification, send an OTP, validate it, but not store the email — requiring re-validation every 3 months?

## Governing law

Sri Lanka's Personal Data Protection Act (PDPA), No. 9 of 2022, as amended. The Data Protection Authority regulates private-sector processing. Email addresses are treated as personal data under the DPA, and processing must have a specified, explicit, legitimate purpose and be limited to what's adequate, relevant, and proportionate for that purpose.

## Assessment: generally sound, with the ephemeral-storage design being a genuine strength

The proposed flow — verify via OTP, extract/confirm the company domain, delete the raw address, store only `company_domain`, `work_email_verified`, `verified_at` — is a privacy-by-design approach the law is built to reward, not an edge case that needs special pleading.

**Important nuance**: not persisting the email doesn't mean no processing occurs. Sending the OTP and reading the domain from the address *is* processing personal data, even transiently. That's fine under PDPA as long as the purpose is specified and proportionate — which "confirm current employment for a verified-professionals community" plausibly is.

**Recommended onboarding copy**: *"Verify your professional email — we use your work email only to confirm that you have access to an email address associated with your organization. Your work email address will not be displayed to other users and will not be retained after verification."*

## The employer question

An employee voluntarily giving the app their work email generally doesn't require employer permission just to send that user a verification message they requested. But the platform should avoid implying employer endorsement: don't show "✓ Verified by Company XYZ" (implies the company vouches for the person); instead show "🟢 Professional Email Verified — Company XYZ" with an on-tap note: *"Verified using an email address associated with Company XYZ. This does not indicate endorsement by Company XYZ."*

## Why 90-day re-verification matters

Someone can verify with `john@company.com` and leave the company two weeks later; without re-verification the platform would keep showing "🟢 Company XYZ" indefinitely — actively misleading. PDPA's accuracy requirement (reasonable steps to keep processed data accurate and current) supports mandatory periodic re-verification. Recommended states: "🟢 Professional Email Verified — verified within last 90 days" vs. "🟡 Professional Email Verification Expired — needs re-verification" (never label the expired state "unverified professional" — the person's employment status may not have changed, only the verification currency).

## Minimize even the operational data

Avoid retaining even a hash of the full email if it isn't needed later. Target retained fields: `user_id`, `company_domain`, `verified`, `verified_at`, `verification_method`. Some short-lived operational data (OTP delivery, abuse prevention, audit/security) may be needed, but define explicit retention periods for it under PDPA's "retain only as long as necessary, protect appropriately" requirement.

## Open legal question — flagged, not resolved

Making work-email verification *mandatory as a condition of using the service at all* raises a consent question: PDPA's consent-freely-given test considers whether service access is conditioned on processing data that isn't strictly necessary for the service. There's a reasonable argument that verification is fundamental to this specific product (a deliberately verified-professionals-only community, not a general app bolting on an unnecessary requirement) — but the legal basis and privacy notice should be structured properly, not just backed by an "I consent" checkbox. **A Sri Lankan privacy lawyer should review this specific point before launch.** Tracked in [Project State](../00-project/project-state.md) § Blocked and referenced from [ADR-003](../04-decisions/adr-003-ephemeral-work-email-verification.md).

## Bottom line

Corporate email verification is feasible and the privacy-by-design approach described (OTP verification, no public email, minimal retention, periodic re-verification) is the direction recommended — not a reason to abandon the approach.

## Related

[ADR-003](../04-decisions/adr-003-ephemeral-work-email-verification.md) · [Verification Model](../02-domain/verification-model.md) · [Privacy & Anti-Abuse Controls](../03-architecture/privacy-and-anti-abuse-controls.md) · [Project State](../00-project/project-state.md)
