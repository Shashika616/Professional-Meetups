# [ADR-009](adr-009-jwt-auth-strategy.md): JWT Auth Strategy — RS256, Refresh Rotation, Platform-Specific Token Storage

**Status:** Accepted

## Context

Two client types need to authenticate against the backend: the Flutter mobile apps (Android/iOS — [ADR-007 - Flutter as the Cross-Platform Frontend](adr-007-flutter-as-the-cross-platform-frontend.md)) and, later, a web-based internal admin/safety dashboard (FR-007 in [Requirements](../01-product/requirements.md), [Operations & Incident Response](../03-architecture/operations-and-incident-response.md)). Multiple independent Cloud Run microservices ([ADR-008 - Backend Platform Architecture](adr-008-backend-platform-architecture.md)) each need to verify caller identity without all sharing one secret.

## Decision

- **Access tokens**: short-lived JWT (~15 minutes), signed with **RS256 (asymmetric)**. Only the auth service holds the private signing key; every other service holds just the public key to verify tokens locally. A leaked or compromised downstream service cannot be used to forge tokens, unlike with a shared HS256 secret.
- **Refresh tokens**: longer-lived, **rotated on every use** — each refresh call issues a new refresh token and invalidates the previous one, tracked in a `refresh_tokens` table (or Redis). A replayed, already-rotated refresh token is detectable and treated as a signal of possible theft, triggering session invalidation. This is also what makes instant logout / force-revocation possible at all, since a bare JWT can't otherwise be revoked before it expires.
- **Mobile clients** (Flutter, iOS/Android): access token held in memory only, never persisted to disk. Refresh token stored in OS-level secure storage (iOS Keychain / Android Keystore) via `flutter_secure_storage`. Not cookies — Flutter's default HTTP clients don't manage cookies the way a browser does, so a cookie-based flow would have to be hand-rolled for no benefit.
- **Web clients** (future admin/safety dashboard): refresh token in an httpOnly, Secure, `SameSite=Strict` cookie. Access-token handling for the dashboard (in-memory JS variable vs. a second cookie) to be finalized when that dashboard is actually built — not needed yet.
- **Staff/admin authentication is a separate system from consumer auth**, not sharing the same JWT issuer, user table, or trust-level model. Likely Google Workspace SSO for staff, given admin accounts are the highest-value target for exactly the session/account-hijacking risks this whole strategy exists to prevent.

## Consequences

- No shared-secret blast radius across services (RS256) — compromising one service's ability to *verify* tokens doesn't give it the ability to *mint* them.
- Refresh-token rotation requires a small piece of persistent state (the rotation-tracking table), which is new infrastructure, not just a JWT library setting.
- Two separate client-side token-storage implementations are needed (mobile secure storage vs. web cookies) — the server-side issuance/verification logic is shared, but there is no single client auth library that covers both.
- Keeping staff auth separate from consumer auth is a deliberate security boundary: a vulnerability or credential leak on the consumer side can never reach admin tooling, and vice versa.

## Related

[ADR-008 - Backend Platform Architecture](adr-008-backend-platform-architecture.md) · [Verification Model](../02-domain/verification-model.md) · [Trust Levels](../02-domain/trust-levels.md) · [Operations & Incident Response](../03-architecture/operations-and-incident-response.md)
