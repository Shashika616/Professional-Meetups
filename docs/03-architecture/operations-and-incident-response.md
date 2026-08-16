# Operations & Incident Response

The human/operational layer behind [Trust & Safety Architecture](trust-and-safety-architecture.md) and [Safety Features Catalog](safety-features-catalog.md) — moderation is not purely automated.

## Human review team

AI moderation alone is insufficient; a small safety team beats none even at MVP. Needs human review: new/unknown company domains, appeals from banned users, serious reports, dating-related abuse, physical-safety reports, identity-mismatch reports, ride-sharing incidents, legal requests, high-risk users, VIP/corporate users.

**SLA targets by severity**:

| Severity | Example | Target response |
|---|---|---|
| Critical | Active threat, stalking, assault, kidnapping risk | Immediate / urgent |
| High | Scam, blackmail, harassment | Within hours |
| Medium | Suspicious profile, fake company | Within 24 hours |
| Low | Minor spam, profile issue | Within 72 hours |

## Safety dashboard (internal)

Track: new fake accounts detected, reports per 1,000 users, match-abuse rate, ride-incident rate, dating-incident rate, ban-evasion attempts, suspicious domains, high-risk messages, response time, verification-failure rate, repeat offenders, regional risk hotspots, company-impersonation attempts, user safety complaints.

## Incident response plan

**Incident types**: user threatened, user assaulted, stalking, scam, blackmail, account takeover, data breach, fake company, minor user discovered, suicide threat, violence threat, ride incident, dating abuse.

**Response steps**: preserve evidence → suspend the suspected account if needed → protect the victim → contact the user if necessary → escalate to the human safety team → contact law enforcement if required → notify affected users if legally required → document the incident → improve controls.

## Working with law enforcement

Prepare in advance: a legal-process policy, data-disclosure rules, an emergency-disclosure process, an evidence-preservation workflow, local legal counsel (and international counsel if operating beyond Sri Lanka), and a user-notification policy where lawful. Don't hand over user data casually — but don't ignore genuine emergency threats either.

## Sri Lanka-specific launch controls

The smaller, more community-driven local professional ecosystem lets the platform start stronger than a global-open launch would allow: start with Colombo and selected professional communities; onboard company-by-company; manually verify top companies; partner with HR departments; combine LinkedIn + work email + phone; require profile-photo consistency; use invite codes early; encourage meetups in public commercial areas; support Sinhala, Tamil, and English reporting; use local moderators who understand cultural context; include local emergency options; be deliberate about gender-based safety; offer women-only or community-filtered options if users want them; allow hiding employer; avoid collecting caste, religion, political affiliation, or other sensitive personal data; treat dating features with extra social-sensitivity care.

## Women's safety as a major design priority

If women don't feel safe, the platform fails. Controls: optional match-only-with-women for certain intents, optional women-only networking spaces, stronger default privacy where chosen, easy block/report, no unsolicited messages from unknown users, no mass messaging, no default appearance-commenting, safety rating, public-meetup recommendations, optional trusted-contact sharing, quick-exit feature, fake-call feature, panic button, immediate human support for serious reports. **Not optional — treat as core to platform viability.**

## Red team exercise (pre-launch)

Before launch, have people actively try to abuse the system: fake LinkedIn profile, fake company domain registration, bypass email verification, multiple accounts, mass messaging, scam links, company impersonation, money requests, unsafe-meetup arrangement, and test the reporting flow, SOS flow, ride flow, dating flow, and ban-evasion paths. Fix whatever they find before opening up further.

## Minimum safety features before launch (MVP checklist)

**Verification**: phone OTP, LinkedIn OAuth, professional email verification, free-email rejection, domain risk check, company allowlist, manual review for unknown companies, device attestation, rate limiting.

**Trust**: new-account restrictions, trust score, limited daily matches, limited messages, no dating initially, no open ride sharing initially.

**Safety UX**: public-meetup recommendations, block button, report button, safety checklist, in-app chat only initially, hide exact location, hide sensitive profile fields, optional trusted-contact sharing.

**Moderation**: AI message-risk detection, link blocking for new users, money-request detection, human review queue, ban appeals, evidence retention.

**Emergency**: SOS button, local emergency-call option, optional live location sharing, check-in feature, incident-response process.

## The strongest defense: closed, trusted growth

For maximum safety at Sri Lanka launch: don't open to everyone immediately. Launch through companies, professional communities, alumni networks, industry associations, an invite-only waitlist, HR partnerships, and LinkedIn-verified professionals. This builds trust and reduces fake accounts; expand publicly later once moderation systems have matured. Recorded as [ADR-005](../04-decisions/adr-005-invite-only-company-based-closed-launch.md).

## Related

[Trust & Safety Architecture](trust-and-safety-architecture.md) · [Safety Features Catalog](safety-features-catalog.md) · [Roadmap](../06-roadmap/roadmap.md) · [ADR-005](../04-decisions/adr-005-invite-only-company-based-closed-launch.md)
