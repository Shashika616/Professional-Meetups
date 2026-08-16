# Privacy & Anti-Abuse Controls

Privacy is safety: over-exposing data is itself an attack surface. Companion to [Safety Features Catalog](safety-features-catalog.md) and [Trust & Safety Architecture](trust-and-safety-architecture.md).

## Location privacy and anti-stalking

- Never show exact home or (unless explicitly opted in) office location; use approximate location and distance, not coordinates, by default. Hide location after a user leaves a meetup. Prevent "always visible" mode for low-trust users. Let users hide from specific people, from their company, or from certain industries, and be invisible except when actively matching.
- **Anti-stalking**: throttle repeated non-mutual searches of the same user; flag a user who appears near another too often; prevent re-contact after a report; prevent blocked users from easily creating new accounts to reach the same victim (device/phone/email/behavior/payment fingerprints, where lawful).

## Minimize public data

Never publicly show: full phone number, exact home/workplace location, daily routine, real-time location to everyone, email address, LinkedIn URL before a match, full name (if the user wants privacy), company name (if hidden), meeting history, or ratings in a way that enables retaliation.

## Data protection baseline

Encrypt data in transit and at rest, restrict admin access, keep audit logs, use role-based access, enforce data-retention limits, secure backups, penetration testing, a bug bounty eventually, secure API rate limits, and protections against scraping, API abuse, and ID enumeration.

## API abuse and scraping prevention

Authentication tokens, device binding, rate limits, pagination/search/profile-view limits, CAPTCHA for suspicious behavior, IP reputation, bot detection, app attestation, signed requests, anomaly detection, honeypot fields, blocked bulk export, unusual-match-behavior detection. No public scraping of professional profiles.

## Detecting organized crime networks

Look for coordinated signals across accounts: same device, same photo, same message templates, same links, same withdrawal/payment details, same phone-number patterns, same location clusters, same writing patterns, same report victims, same external-contact handles, same company domains, same LinkedIn connection patterns. Use graph analysis, not single-account review, to catch these.

## Company-search abuse prevention

"Search by company" (Premium — see [Revenue Model](../01-product/revenue-model.md)) can be abused by stalkers, sales spammers, recruiters spamming candidates, harassers, or corporate-espionage actors. Controls: let users hide their company or block discovery by company, limit search volume, prevent saving/exporting results, prevent mass-messaging a company's employees, rate-limit, require higher trust for the feature.

## Fake-events prevention (if/when events ship)

Only verified users can create events; public events require review; event location must be public/approved; RSVP happens in-app; hosts need a trust-score threshold; post-event safety feedback collected; detect fake event names and impersonation of real companies/events.

## Data harvesting prevention

Prevent bulk profile viewing and scraping, limit export, rate-limit search, detect abnormal behavior, hide LinkedIn/email until mutual match, discourage/detect screenshots and screen recording where feasible, warn users against sharing sensitive documents.

## Do not store unnecessary sensitive documents

For any ID, pay slip, employee card, or vehicle document collected: collect only what's necessary, redact unnecessary parts, store encrypted, restrict access, delete after verification where possible, use trusted eKYC providers where available, comply with data-protection law, get explicit consent. **The more data stored, the bigger the risk** — this is the same principle behind [ADR-003](../04-decisions/adr-003-ephemeral-work-email-verification.md)'s "verify, don't retain" approach to work email.

## Visible safety score (not the raw trust score)

Show a simple tiered indicator matching [Trust Levels](../02-domain/trust-levels.md) (LinkedIn Connected / Personal Verified / Corporate Verified / High Trust Member) plus member-since date, successful-meetup count, and response rate — never the exact internal trust score, and avoid over-gamifying trust.

## Related

[Trust & Safety Architecture](trust-and-safety-architecture.md) · [Safety Features Catalog](safety-features-catalog.md) · [Verification Model](../02-domain/verification-model.md) · [Sri Lanka PDPA - Work Email Verification](../07-research/sri-lanka-pdpa-work-email-verification.md) · [ADR-003](../04-decisions/adr-003-ephemeral-work-email-verification.md)
