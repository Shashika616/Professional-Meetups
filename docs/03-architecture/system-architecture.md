# System Architecture

**Status: structural only — no technology stack has been chosen yet** (see [Project State](../00-project/project-state.md) § Blocked). This note captures the shape the system must have to satisfy [Requirements](../01-product/requirements.md); fill in concrete tech choices as ADRs once decided.

## Components implied by requirements

- **Onboarding & verification service** — phone OTP, LinkedIn OAuth, work-email domain verification, device attestation. See [Verification Model](../02-domain/verification-model.md).
- **Trust engine** — computes trust level (0–4) and continuous trust score; gates feature access. See [Trust Levels](../02-domain/trust-levels.md), [Trust & Safety Architecture](trust-and-safety-architecture.md).
- **Matching engine** — real-time geospatial + compatibility matching on Intent. Must return results in <200ms (NFR-001).
- **Messaging service** — in-app chat with automated scam/risk detection, link blocking for new/low-trust accounts. See [Safety Features Catalog](safety-features-catalog.md) § Communication Safety.
- **Meetup/Safety-gate service** — orchestrates the pre-meetup checklist, public-venue suggestions, check-in timers, and post-meetup feedback. See [Safety UX Flows](../05-ux/safety-ux-flows.md).
- **SOS/emergency service** — panic button, trusted-contact notification, location-based emergency-number lookup.
- **Company/domain intelligence service** — domain age/DNS/MX/SPF-DKIM-DMARC checks, company allowlist, manual-review queue for unknown domains.
- **Moderation/anti-abuse service** — device fingerprinting, ban-evasion detection, message risk scanning, organized-crime graph analysis. See [Privacy & Anti-Abuse Controls](privacy-and-anti-abuse-controls.md).
- **Enterprise admin panel** — HR-facing dashboard for employee onboarding, domain verification, commuting-group monitoring.
- **Internal safety dashboard** — for the human review/trust & safety team. See [Operations & Incident Response](operations-and-incident-response.md).

## Non-functional constraints that shape architecture

- **NFR-001 Performance** — matching engine <200ms → likely needs a fast geospatial index, not a naive DB query per match.
- **NFR-002 Scalability** — horizontal scaling for commute-hour spikes → stateless services, queue-backed matching where possible.
- **NFR-003 Privacy** — locations must be fuzzed until mutual confirmation; E2EE on private messages → architecture must support message-content opacity to the backend where feasible, or at minimum strict access control + audit logging.
- **NFR-004 Compliance** — PDPA/GDPR → minimal-retention data model, especially for work-email verification (see [ADR-003](../04-decisions/adr-003-ephemeral-work-email-verification.md)) and sensitive documents (see [Privacy & Anti-Abuse Controls](privacy-and-anti-abuse-controls.md) § Do Not Store Unnecessary Sensitive Documents).
- **NFR-005 Availability** — 99.9% uptime with automated failover specifically for SOS/emergency paths — this likely needs its own resilience budget separate from the rest of the system.

## Data sensitivity note

Two data classes need architecturally distinct handling: (1) short-lived verification material (raw work email, ID photos, vehicle documents) that should be deleted or heavily restricted post-verification, and (2) long-lived low-sensitivity derived data (company_domain, trust score, verification booleans) that's safe to keep. Don't let them live in the same table/service with the same retention policy.

## Related

[Requirements](../01-product/requirements.md) · [Domain Model](../02-domain/domain-model.md) · [Trust & Safety Architecture](trust-and-safety-architecture.md) · [Project State](../00-project/project-state.md)
