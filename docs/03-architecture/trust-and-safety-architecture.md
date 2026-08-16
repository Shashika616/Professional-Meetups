# Trust & Safety Architecture

The core defense system for the platform. Built from [Threat Model](threat-model.md) and expressed through [Trust Levels](../02-domain/trust-levels.md) and [Verification Model](../02-domain/verification-model.md). Companion notes: [Safety Features Catalog](safety-features-catalog.md) (per-intent and per-feature controls) and [Privacy & Anti-Abuse Controls](privacy-and-anti-abuse-controls.md) (data-exposure and platform-abuse controls).

## Core principle

Do not trust any single verification method — see [Verification Model](../02-domain/verification-model.md) for why phone, LinkedIn, email, profile details, and profile photos are each individually defeatable. The defense is layered verification + continuous trust scoring + safety controls, never one signal alone.

## Continuous trust score

Verification is not one-time. Every user carries a dynamic trust score built from ongoing signals, layered on top of the discrete [Trust Levels](../02-domain/trust-levels.md).

**Positive signals**: verified phone/LinkedIn/professional email, known company domain, account age, complete profile, positive ratings, successful meetups, no reports, consistent location behavior, normal messaging behavior, mutual professional connections, enterprise verification, attended verified events.

**Negative signals**: newly created LinkedIn, recently registered domain, multiple failed OTP attempts, multiple accounts from the same device, reports from other users, aggressive messaging, asking for money, sending suspicious links, trying to move the conversation off-platform too quickly, inconsistent location, profile-photo mismatch, multiple blocked attempts, sudden behavior change, login from high-risk geography, complaints after meetups.

**Access gating by score band**:
- *Low trust* → fewer matches, no dating, no ride sharing, no search, no premium visibility, limited messaging.
- *Medium trust* → normal networking access, limited intents.
- *High trust* → more features, corporate features, event hosting, mentorship, ride sharing if further checks pass.

## New-account slowdown

New accounts do not get full access immediately — one of the simplest, strongest defenses. For the first 7–30 days: limited daily matches, limited messages/day, no dating intent, no ride-sharing intent, no private meetup requests, no search by company, no bulk profile browsing, no external link sharing, no file sharing, no phone-number sharing, no large-audience "Available Now" broadcasting. Restrictions may be reduced for corporate/enterprise-verified accounts.

## Device-level fraud prevention

See [Verification Model](../02-domain/verification-model.md) § 4 for the full control list (fingerprinting, app attestation, VPN handling, etc.).

## Related

[Threat Model](threat-model.md) · [Trust Levels](../02-domain/trust-levels.md) · [Verification Model](../02-domain/verification-model.md) · [Safety Features Catalog](safety-features-catalog.md) · [Privacy & Anti-Abuse Controls](privacy-and-anti-abuse-controls.md) · [ADR-002](../04-decisions/adr-002-multi-level-continuous-trust-scoring.md)
