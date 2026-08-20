# Multi-stage build: compile in a full Go image, ship only the static binary
# in a minimal runtime image. Keeps the production image small (faster Cloud
# Run cold starts, matters for ADR-008's cost model) and free of build tools.
#
# Build context is the `backend/` directory (see docker-compose.yml's
# `context: .` for this service), not this service's own folder, because the
# module needs access to ../../shared via the go.work workspace.

FROM golang:1.26.6-alpine AS build
WORKDIR /workspace

# Copy the whole workspace (shared + all services' go.mod/go.sum) before
# source, so dependency downloads are cached across builds unless go.mod
# actually changes.
COPY go.work go.work.sum* ./
COPY shared/go.mod shared/go.sum* ./shared/
COPY services/gateway/go.mod services/gateway/go.sum* ./services/gateway/
# go.work lists all four modules, so `go work sync` needs every one of
# their go.mod files present even though this image only builds gateway.
COPY services/auth/go.mod services/auth/go.sum* ./services/auth/
COPY services/meetup/go.mod services/meetup/go.sum* ./services/meetup/
RUN go work sync

COPY shared ./shared
COPY services/gateway ./services/gateway

WORKDIR /workspace/services/gateway
# CGO disabled + static build: the runtime stage below has no C libraries.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/gateway ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot AS runtime
COPY --from=build /out/gateway /gateway
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/gateway"]
