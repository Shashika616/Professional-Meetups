// Shared code used by both services: JWT issuance/verification (ADR-009),
// generated gRPC/protobuf code, structured logging setup, common error
// types. Nothing here should import from services/gateway or services/auth —
// dependencies only flow one way, services depend on shared, never the
// reverse. See backend/PLAN.md for what goes in each package.
module github.com/professional-connections/backend/shared

go 1.26.6

require (
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/google/uuid v1.6.0
	google.golang.org/grpc v1.83.0
	google.golang.org/protobuf v1.36.12
)

require (
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260526163538-3dc84a4a5aaa // indirect
)
