# Safety Features Catalog

Per-intent and per-feature safety controls. Companion to [Trust & Safety Architecture](trust-and-safety-architecture.md) (the scoring/verification core) and [Threat Model](threat-model.md) (what we're defending against).

## Intent-based safety rules

Each **Intent** (see [Domain Model](../02-domain/domain-model.md)) carries different risk and different required controls:

- **Low-risk** (coffee, lunch, professional networking, mentorship, startup discussion, industry community): public-meeting recommendation, in-app chat only, mutual match required, basic verification.
- **Medium-risk** (breakfast, after-work drinks, event meetup, community gathering): public-venue recommendation, optional check-in, higher trust score, user ratings.
- **High-risk** (dating, ride sharing, late-night meetups, home/office pickup, private-location meetups): higher trust level, account-age requirement, optional ID verification, live location sharing, emergency button, meeting check-in, ride-specific vehicle verification, stronger moderation, two-way consent, possibly limited to users with prior positive meetup history.

## Ride-sharing safety

Ride sharing is one of the highest-risk features proposed — treat it almost as a separate safety-critical product (deferred to a later phase; see [ADR-004](../04-decisions/adr-004-defer-dating-and-open-ride-sharing.md)).

- **Driver verification**: phone, LinkedIn or professional email, government ID, driving license, vehicle registration, vehicle insurance, vehicle photo, license-plate verification, driver-photo match, optional criminal background check where legal, optional employer verification for corporate commuting.
- **Passenger verification**: verified account, trust score above threshold, no recent abuse reports, in-app identity confirmation, optional emergency-contact sharing.
- **Ride safety features**: live route tracking, share-ride-with-trusted-contact, in-app SOS, route-deviation alerts, expected arrival time, driver/passenger ratings, check-in before start, check-out after finish, anonymized phone numbers, no cash handling where possible, no off-platform communication before ride confirmation, cancel-without-penalty if unsafe, public (not private) pickup points.
- **Smart pickup recommendations**: never reveal exact home location — suggest building lobby, near main gate, public landmark, office reception, fuel station, supermarket entrance, public parking. Exact location only after both parties confirm.

## Dating mode needs extra protection

**Recommendation: don't launch dating immediately.** If launched later, keep it separate and highly controlled (see [ADR-004](../04-decisions/adr-004-defer-dating-and-open-ride-sharing.md)).

- **Controls**: 18+ only, explicit opt-in, separate dating-profile visibility (only shown to users who also opted into dating), strong reporting/block tools, no explicit image sharing by default, sextortion-pattern detection, romance-scam language detection, warnings on money requests or premature off-platform moves, higher trust-level requirement, optional ID/video verification before first date, public-first-date recommendation, date check-in, trusted-contact sharing, fake-call/exit feature, safety timer.
- **Red flags to auto-detect**: requests for money, gift cards, crypto investment talk, OTP, bank details, private photos; threats; emotional-manipulation patterns; rapid escalation; multiple reports from different users; duplicate profile photos across accounts.

## Communication safety

Most scams happen through messages.

- **Messaging restrictions for new/low-trust users**: limit messages/day and new matches/day, block mass messaging, block copy-paste messages, block external links, block phone numbers/emails until trust threshold reached, block file attachments by default, block payment requests, block QR codes where possible, detect suspicious URLs.
- **Automated message-risk detection**: scam patterns, phishing/malware links, investment fraud, job scams, recruitment-fee requests, money requests, sexual harassment, threats, blackmail, hate speech, impersonation, spam. If scanning message content, disclose it clearly and comply with data protection law.

## Making off-platform migration harder

Criminals move victims to WhatsApp/Telegram/Signal/SMS/email to avoid detection. Can't fully prevent contact-sharing, but can: warn when external contact sharing is detected, restrict it for new accounts, require a mutual trust threshold before sharing phone/email, show an in-context safety warning, flag repeated off-platform-migration attempts, require both users to confirm comfort before moving off-platform. Frame this as safety, not punishment.

## Meeting safety (the Safety Gate)

See [Safety UX Flows](../05-ux/safety-ux-flows.md) for the full step-by-step flow. Summary:

- **Before meeting**: show first name only by default, verified badge, trust level, optionally-hidden company, approximate location, mutual communities, previous ratings, safety checklist. Recommend public daytime venues (cafe in hotel/office building, restaurant public area, co-working lobby, transit hub, office reception); discourage private homes, hotel rooms, remote locations, late-night first meetings, car pickups (unless ride-sharing verified).
- **During meeting**: optional check-in button, safety timer, live location to trusted contact, fake-incoming-call feature, quick-exit help, emergency SOS, "I am safe" / "I need help" buttons.
- **After meeting**: ask both parties whether the meeting happened, whether they felt safe, whether the profile was accurate, whether they'd meet again, and whether they want to report. Feeds the trust score; detailed feedback is internal-only, never shown publicly.

## Reporting and blocking

Must be extremely easy — reporting should take seconds.

- **Report reasons**: fake profile, impersonation, scam, asked for money, suspicious link, harassment, threats, sexual harassment, stalking, unsafe meeting, unsafe ride, identity mismatch, minor user, violence, theft, other.
- **After a report**: immediate block, immediate restrict, hide profile, cancel upcoming meetup, stop location sharing, contact support, preserve evidence.
- **Evidence preservation**: retain necessary metadata per the retention policy even if a user deletes the conversation — don't let bad actors delete evidence too easily.

## Ban evasion prevention

Signals: device fingerprint, phone number, email domain, name patterns, profile-photo hash, behavioral patterns, payment method, IP address, geolocation patterns, social-graph connections, writing-style patterns (where lawful), LinkedIn linkage, company domain. Response options: ban, shadow restriction, reduced visibility, forced re-verification, manual review, temporary/permanent suspension. Don't always disclose exactly why an account was flagged — that helps attackers adapt.

## Financial and job-scam prevention

- **Prohibited**: money requests, loan offers, investment guarantees, crypto schemes, pyramid schemes, job/recruitment fees, fake-cheque scams, gift-card requests, advance-fee fraud, charity scams, emergency-money requests.
- **Automated warnings** trigger on patterns like "send me money," "guaranteed profit," "pay registration fee," "send OTP," "your account will be blocked" — surfaced as an in-context warning, not a silent block.
- **Job-scam red flags**: job offer without interview, high pay for easy work, training/equipment fee requests, early bank-detail requests, free-email recruiter contact, unsolicited offers, pressure to move to Telegram immediately. Controls: stronger recruiter verification, required company domain, corporate verification for hiring intent, rate limits on outreach messages, mass-recruitment-message detection, dedicated report option.

## Impersonation prevention

Attackers may pose as CEOs, bank officers, government officials, recruiters from famous companies, investors, or HR managers. Controls: review gate on name changes, detect famous-company claims on unverified profiles, require official domain for company claims, flag unverified executive-role claims, badges only after domain/company verification (never user-customizable), easy impersonation reporting.

## Enterprise mode — the safer growth path

Verification options: company SSO, SAML/OIDC, Azure AD/Google Workspace/Okta, HR-approved user list, admin-invite-only onboarding, company-domain verification, employee-ID validation, internal directory sync, role-based access. Safety benefits: employer-verified users, lower-risk internal networking, company-specific commuting groups, reports routable to the employer's admin, employer-controlled badges, instant offboarding on employee exit. Especially strong fit for Sri Lanka's corporate market — see [Revenue Model](../01-product/revenue-model.md).

## "Available Now" misuse prevention

Limit who can see it, limit broadcast frequency, never broadcast to everyone (compatible matches only), hide exact location, require mutual intent, restrict new users from using it, detect repeated failed meetup attempts or harassment use.

## Do not over-trust "verified" badges

A verified badge means "this account passed certain checks," never "this person is safe." Always pair it with safety messaging: *"Verified does not guarantee good intentions. Always meet in public and trust your instincts."* If users conflate verified with safe, they take more risk than they should.

## SOS / emergency flow

Options: call local emergency services, share live location with trusted contacts, notify the app safety team, start recording where legally allowed, trigger a fake call, show safety instructions, cancel the meetup instantly, stop all location sharing with the matched user, preserve evidence. Emergency numbers vary by country — use location-based emergency-number detection, don't hardcode one country.

## Trusted contact sharing

Optional but strongly encouraged. Shareable details: matched person's first name, verification status, meeting place, time, live location, expected end time, emergency check-in link.

## Verification should be hard to buy

Premium tier must never sell trust — see [Revenue Model](../01-product/revenue-model.md) for the specific guardrail.

## Related

[Trust & Safety Architecture](trust-and-safety-architecture.md) · [Threat Model](threat-model.md) · [Privacy & Anti-Abuse Controls](privacy-and-anti-abuse-controls.md) · [Safety UX Flows](../05-ux/safety-ux-flows.md) · [Operations & Incident Response](operations-and-incident-response.md) · [ADR-004](../04-decisions/adr-004-defer-dating-and-open-ride-sharing.md)
