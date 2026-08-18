# Action Tracker

**Single source of truth for "what's outstanding" — checked and updated every session, not just written once.** [Project State](project-state.md) is the narrative history (what happened, when, why); this note is the checklist (what's still open, who owns it, is it blocking). If something here conflicts with [Project State](project-state.md) or an ADR's original text, this note wins — it reflects the latest correction, not the first draft. Read this before assuming anything is either "done" or "still undecided."

**Anyone reading this should be able to answer**: what needs Shashika's action right now, what's blocking Claude Code, and what's a known gap we're deliberately not building yet.

Last swept end-to-end: 2026-08-17.

---

## 1. Needs Shashika's action right now (not blocking current build)

- [ ] **Buy a domain and verify it with Resend** — not blocking; testing proceeds via `LoggingEmailSender` or the `onboarding@resend.dev` sandbox (own-inbox-only) in the meantime. See [ADR-012 - Level 2-3 Verification Delivery and Identity Anchors](../04-decisions/adr-012-level-2-3-verification-delivery-and-identity-anchors.md)'s correction section for the exact DNS-record steps once purchased.
- [ ] **Create a Twilio account and fill in `TWILIO_ACCOUNT_SID`/`TWILIO_AUTH_TOKEN`/`TWILIO_PHONE_NUMBER` in `backend/.env`** — not yet done as of this sweep (only the Resend API key has been created so far). Not blocking — `LoggingSmsSender` fallback works until then.
- [ ] **Fill in `RESEND_FROM_EMAIL`** with a real domain address once the domain above is verified (currently defaulted to the sandbox address in `.env.example`).

## 2. Real decisions only Shashika can make (genuinely blocking eventual launch, not today's build)

- [ ] **Level 1b product rule**: confirm a pasted-URL-only LinkedIn ("claimed," not federated) should stay unable to match/be matched, per [ADR-006](../04-decisions/adr-006-progressive-linkedin-first-trust-onboarding.md)'s default. Real trade-off — some users who won't do OAuth can browse but never actually use the app.
- [ ] **Legal review (Sri Lanka PDPA)**: a Sri Lankan privacy lawyer needs to review whether making LinkedIn/personal verification *mandatory* to use the service at all is compliant — consent-conditionality question, not yet resolved. Flagged in [ADR-003](../04-decisions/adr-003-ephemeral-work-email-verification.md) and [Sri Lanka PDPA - Work Email Verification](../07-research/sri-lanka-pdpa-work-email-verification.md).
- [ ] **Account-deletion retention tension**: soft-delete + PII anonymization (agreed direction) conflicts with Ban Evasion Prevention needing *some* non-reversible fingerprint retained post-"deletion." Same legal-review category as above.
- [ ] **Company allowlist ownership**: who manually curates the pre-verified company database ([Verification Model](../02-domain/verification-model.md) § Company Verification Database)? Nobody assigned yet. Not urgent — this addendum's corporate-email MVP has no allowlist at all yet (see § 4 below).
- [ ] **GCP lock-in acknowledgment**: [ADR-008](../04-decisions/adr-008-backend-platform-architecture.md) commits Pub/Sub + Cloud Run IAM + Cloud Run ingress specifically. Worth a conscious yes rather than a surprise later if multi-cloud ever comes up.
- [ ] **Deployment region**: Mumbai vs. Singapore for Cloud Run + Cloud SQL. Needs a real latency test against NFR-001's <200ms target, not a guess.
- [ ] **RPO/RTO targets for Postgres**: [ADR-010](../04-decisions/adr-010-postgresql-hosting-on-cloud-sql.md) turns on backups/PITR, but "backups are on" isn't the same as an agreed data-loss/downtime tolerance.
- [ ] **Re-linking a different LinkedIn account** to an existing phone/email-anchored account — [ADR-012](../04-decisions/adr-012-level-2-3-verification-delivery-and-identity-anchors.md) explicitly left this undesigned.

## 3. Blocking Claude Code right now

Nothing. The Level 2/3 verification addenda (`backend/PLAN.md`, `frontend/PLAN.md`) are fully specified, audited, and ready to hand off — see the current prompt in the latest chat turn. Both Twilio and Resend fall back to logging senders, so empty credentials don't block a build or test run.

**Sequencing note, still live**: don't hand off the Level 2/3 addenda until the in-flight Steps 12-15 diff (session UX / homepage personalization) is reviewed and merged — both touch `app_providers.dart`/`ProfilePage`.

## 4. Known gaps — deliberately deferred, not forgotten

Corporate email (Level 3) MVP, per [ADR-012](../04-decisions/adr-012-level-2-3-verification-delivery-and-identity-anchors.md):
- [ ] Domain-age/SPF/DKIM/DMARC checks
- [ ] Company verification database (pre-verified known companies)
- [ ] Manual review flow for unknown domains
- [ ] 90-day work-email re-verification *job* (only its data dependency, `work_email_verified_at`, is being built)
- [ ] Display copy for a verified corporate badge — [ADR-003](../04-decisions/adr-003-ephemeral-work-email-verification.md) requires "Professional Email Verified — Company XYZ" with an on-tap non-endorsement disclaimer, never "✓ Verified by Company XYZ" — not yet built anywhere in the UI (no corporate-verified badge exists yet).
- [ ] **Named risk (flagged 2026-08-17 by Shashika, now documented in [Verification Model](../02-domain/verification-model.md) § 5 and [ADR-012](../04-decisions/adr-012-level-2-3-verification-delivery-and-identity-anchors.md)): fresh lookalike domains pass this MVP's corporate check cleanly.** Buy any domain that reads as corporate (`acme-hr.com` for real company `acme.com`), set up any non-role-based address on it, complete the OTP — done, "work email verified," with zero check that the domain belongs to a real employer. Accepted for MVP (Level 3 doesn't gate safety), but `frontend/PLAN.md`'s Level 2/3 addendum now has a hard copy requirement so the badge doesn't overclaim ("email verified," never "company verified"). **Cheapest future mitigation, not yet scoped as its own task**: a simple domain-age/WHOIS check would catch same-day-registered domains without needing the full company database or manual review — worth considering as a smaller intermediate step before building the full deferred stack, if this risk becomes a real problem in practice.

Backend production-readiness (tracked since 2026-08-16, still open):
- [ ] Secrets management — GCP Secret Manager assumed, not configured. JWT private key, LinkedIn secret, Twilio/Resend keys, DB credentials all currently just live in `.env`/`secrets/` locally.
- [ ] Backend CI/CD — no pipeline builds/pushes/deploys the Go services to Cloud Run yet. Only the Flutter frontend has CI (`flutter-ci.yml`).
- [ ] Rate limiting / WAF — Cloud Armor not evaluated; current limiter is app-level Redis only.
- [ ] Infra/AppSec threat coverage — container image CVEs, dependency scanning, IAM misconfiguration: a distinct category from the product threat model, not started.
- [ ] Backend test strategy — no formal decision on unit/integration/contract testing coverage targets across services.
- [ ] Cost monitoring — no GCP budget alerts configured.
- [ ] Realtime Gateway connection-registry design — the general shape is decided ([ADR-008](../04-decisions/adr-008-backend-platform-architecture.md)), the actual Redis-backed `user_id → instance_id` routing mechanism is not.
- [ ] `subscriptions` table — shape decided ([Domain Model](../02-domain/domain-model.md) § Subscription), not built; correctly out of scope until Premium-tier work starts.

Product features explicitly out of scope for now:
- [ ] KYC / Trust Level 4 — not built, Phase 3 per [Roadmap](../06-roadmap/roadmap.md).
- [ ] Meetup creation/scheduling, Matching engine, Safety Gate flow, Realtime/chat — a different domain entirely (own backend service per [ADR-008](../04-decisions/adr-008-backend-platform-architecture.md)), scoped as its own future ADR + PLAN.md once Level 2/3 verification lands. Not started.
- [ ] Dating mode, open ride-sharing — deferred to Phase 2/3 by [ADR-004](../04-decisions/adr-004-defer-dating-and-open-ride-sharing.md), deliberately.
- [ ] iOS/Android permission usage-strings (location/camera) — needed before, not during, SOS/live-location/KYC work. Not yet added.
- [ ] Company/employer name display anywhere in the UI — LinkedIn's self-serve product doesn't expose it; would need LinkedIn Partner Program approval (weeks-to-months). Explicitly deferred, not self-reported either.

## 4b. Housekeeping found during this sweep

- [x] `CLAUDE.md`'s Navigation diagram and "frontend-only scaffold" framing were stale (described the pre-[ADR-011](../04-decisions/adr-011-linkedin-onboarding-slice-design.md) 4-step wizard and all-mock services) — fixed the most actively-misleading lines and added a pointer to this tracker. **Still open**: a fuller pass over `CLAUDE.md`'s Architecture section (Trust-level gating, Feature folder layout, etc.) hasn't been re-verified against current code line by line — do that the next time a frontend architecture question comes up, don't assume every sentence in that section is current.

## 5. Recently corrected — for awareness, already resolved

- [ADR-011](../04-decisions/adr-011-linkedin-onboarding-slice-design.md): PKCE removed from the LinkedIn OAuth flow (LinkedIn's self-serve OIDC product doesn't support it — confirmed via direct testing). Fixed in both frontend and backend, `state`-based CSRF check kept.
- [ADR-012](../04-decisions/adr-012-level-2-3-verification-delivery-and-identity-anchors.md): phone verification moved off Firebase Phone Auth onto the same backend-owned OTP mechanism as email (Twilio plain Messaging API, not Twilio Verify — chosen deliberately to keep one unified mechanism rather than reintroducing a second one).
- Gateway info-disclosure fix (raw internal/upstream errors reaching API clients) — confirmed fixed in `apperror.ToGRPCStatus` and `service.go`.
- Single-identity-anchor risk — resolved via `phone_number`/`personal_email` `UNIQUE` constraints ([ADR-012](../04-decisions/adr-012-level-2-3-verification-delivery-and-identity-anchors.md)).

## Related

[Project State](project-state.md) (narrative history) · [Roadmap](../06-roadmap/roadmap.md) (feature sequencing) · `04 - Decisions` (all ADRs, the *why* behind each item above)
