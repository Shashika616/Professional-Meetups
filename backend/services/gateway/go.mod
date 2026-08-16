// The public entry point (ADR-008): terminates the client-facing REST API,
// verifies JWTs, rate-limits, and translates REST calls into gRPC calls
// against internal services. Deployed as its own Cloud Run service — this
// module must stay independently buildable without the other services.
module github.com/professional-connections/backend/services/gateway

go 1.26.6

require (
	github.com/professional-connections/backend/shared v0.0.0
	github.com/redis/go-redis/v9 v9.22.0
	google.golang.org/grpc v1.83.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260526163538-3dc84a4a5aaa // indirect
	google.golang.org/protobuf v1.36.12 // indirect
)

replace github.com/professional-connections/backend/shared => ../../shared
