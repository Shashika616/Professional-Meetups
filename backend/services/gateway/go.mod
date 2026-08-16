// The public entry point (ADR-008): terminates the client-facing REST API,
// verifies JWTs, rate-limits, and translates REST calls into gRPC calls
// against internal services. Deployed as its own Cloud Run service — this
// module must stay independently buildable without the other services.
module github.com/professional-connections/backend/services/gateway

go 1.23

require github.com/professional-connections/backend/shared v0.0.0

replace github.com/professional-connections/backend/shared => ../../shared
