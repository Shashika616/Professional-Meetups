// Shared code used by both services: JWT issuance/verification (ADR-009),
// generated gRPC/protobuf code, structured logging setup, common error
// types. Nothing here should import from services/gateway or services/auth —
// dependencies only flow one way, services depend on shared, never the
// reverse. See backend/PLAN.md for what goes in each package.
module github.com/professional-connections/backend/shared

go 1.23
