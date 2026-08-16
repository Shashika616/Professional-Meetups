# Threat Model

Attack scenarios the platform must defend against, informing every control in [Trust & Safety Architecture](trust-and-safety-architecture.md), [Safety Features Catalog](safety-features-catalog.md), and [Privacy & Anti-Abuse Controls](privacy-and-anti-abuse-controls.md).

## A. Fake professional identity

Attacker creates a fake LinkedIn profile + fake company email domain + fake phone number + stolen/AI-generated photo, to scam users, extract money, phish professionals, collect corporate information, or harass/blackmail.

## B. Stolen LinkedIn or email account

Attacker compromises a real LinkedIn account or real professional email account to impersonate a genuine professional, bypass verification, and gain trust quickly.

## C. Fake company domain

Attacker registers lookalike domains (`company-careers.com`, `company-jobs.net`, `company-hr.org`, `company-lk.com`) to appear like a legitimate employer, target job seekers, or run recruitment scams.

## D. Social engineering

Attacker builds trust over time, then asks for money, requests investment, sends malicious links, asks for OTP codes, requests personal documents, or tries to move the conversation off-platform.

## E. Physical safety threats — the most serious category

Stalking, assault, robbery, kidnapping, harassment, unsafe ride-sharing situations. This category drives most of [Safety UX Flows](../05-ux/safety-ux-flows.md) and the SOS/emergency design.

## F. Ride-sharing abuse

Fake rides, tracking users, unsafe pickups, identity theft, fraudulent payment requests, gender-based harassment. See [Safety Features Catalog](safety-features-catalog.md) § Ride-Sharing Safety.

## G. Dating-mode abuse

Romance scams, sexual extortion, fake relationships for financial fraud, impersonation, coercion or abuse. See [Safety Features Catalog](safety-features-catalog.md) § Dating Mode Needs Extra Protection.

## H. Corporate espionage

Attackers pose as professionals to extract confidential business information, recruit insiders, pitch fake partnerships, or distribute malware/phishing links.

## Related

[Trust & Safety Architecture](trust-and-safety-architecture.md) · [Safety Features Catalog](safety-features-catalog.md) · [Privacy & Anti-Abuse Controls](privacy-and-anti-abuse-controls.md) · [Verification Model](../02-domain/verification-model.md)
