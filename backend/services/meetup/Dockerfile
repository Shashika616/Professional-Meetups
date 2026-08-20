# Same shape as services/auth/Dockerfile — see the comments there. Build
# context is `backend/` (see docker-compose.yml), not this folder.

FROM golang:1.26.6-alpine AS build
WORKDIR /workspace

COPY go.work go.work.sum* ./
COPY shared/go.mod shared/go.sum* ./shared/
COPY services/meetup/go.mod services/meetup/go.sum* ./services/meetup/
# go.work lists all four modules, so `go work sync` needs every one of
# their go.mod files present even though this image only builds meetup.
COPY services/auth/go.mod services/auth/go.sum* ./services/auth/
COPY services/gateway/go.mod services/gateway/go.sum* ./services/gateway/
RUN go work sync

COPY shared ./shared
COPY services/meetup ./services/meetup

WORKDIR /workspace/services/meetup
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/meetup ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot AS runtime
COPY --from=build /out/meetup /meetup
USER nonroot:nonroot
EXPOSE 9091
ENTRYPOINT ["/meetup"]
