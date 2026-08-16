// The onboarding/auth service (ADR-006, ADR-009): LinkedIn OIDC exchange,
// user record creation, JWT issuance, refresh-token rotation. Deployed as
// its own Cloud Run service, reachable only via gRPC from the gateway —
// this module must stay independently buildable without the other services.
module github.com/professional-connections/backend/services/auth

go 1.23

require github.com/professional-connections/backend/shared v0.0.0

replace github.com/professional-connections/backend/shared => ../../shared
