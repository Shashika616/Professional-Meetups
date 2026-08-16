# Same shape as services/gateway/Dockerfile — see the comments there. Build
# context is `backend/` (see docker-compose.yml), not this folder.

FROM golang:1.23-alpine AS build
WORKDIR /workspace

COPY go.work go.work.sum* ./
COPY shared/go.mod shared/go.sum* ./shared/
COPY services/auth/go.mod services/auth/go.sum* ./services/auth/
RUN go work sync

COPY shared ./shared
COPY services/auth ./services/auth

WORKDIR /workspace/services/auth
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/auth ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot AS runtime
COPY --from=build /out/auth /auth
USER nonroot:nonroot
EXPOSE 9090
ENTRYPOINT ["/auth"]
