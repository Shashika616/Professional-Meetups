# Professional Connections — Backend

Go microservices on Cloud Run (production) / Docker Compose (local dev). See
`docs/04-decisions/adr-008` through `adr-011` (or the vault, if you're
reading this from Cowork's side) for why this shape was chosen — this README
is "how to run it," not "why it's built this way."

**Status:** scaffolding only. Folder structure, Docker setup, SQL schema, and
the gRPC contract are in place; the actual Go source hasn't been written yet.
See `PLAN.md` for the exact build plan.

## Prerequisites

1. **Go** (for fast local iteration — `go build`/`go test` without a Docker
   rebuild every time; the stack itself doesn't strictly require this since
   everything also builds inside Docker, but it makes development much
   faster). Install via Homebrew: `brew install go`, then confirm with
   `go version`.
2. **Docker Desktop**, already installed.
3. A LinkedIn OAuth app, registered at
   [linkedin.com/developers/apps](https://www.linkedin.com/developers/apps),
   with the "Sign In with LinkedIn using OpenID Connect" product added.
4. A local RSA keypair for JWT signing (see below) — not committed, ever.

## First-time setup

```bash
cd backend
cp .env.example .env
# edit .env: fill in LINKEDIN_CLIENT_ID / LINKEDIN_CLIENT_SECRET from the
# LinkedIn app you registered above.

mkdir -p secrets
openssl genrsa -out secrets/jwt_private.pem 2048
openssl rsa -in secrets/jwt_private.pem -pubout -out secrets/jwt_public.pem
# secrets/ is gitignored — this keypair never leaves your machine. Production
# uses GCP Secret Manager instead (tracked as an open item, see Project State).

docker compose up --build
```

This starts Postgres, Redis, the Pub/Sub emulator, runs migrations, and
builds/starts the gateway (`localhost:8080`) and auth service (internal
gRPC, exposed on `localhost:9090` for local debugging only).

## Running the Flutter app against this stack

- **iOS simulator**: point the app at `http://localhost:8080` — the
  simulator shares the host's network stack.
- **Android emulator**: point the app at `http://10.0.2.2:8080` instead —
  the Android emulator has its own virtual network and `localhost` inside it
  refers to the emulator itself, not your Mac. `10.0.2.2` is the emulator's
  alias for the host machine. This trips people up constantly; if requests
  are mysteriously timing out on Android but not iOS, check this first.
- **Physical device**: needs your Mac's LAN IP instead of `localhost`, and
  the device on the same network — not needed yet at this stage.

## Common commands

```bash
docker compose up --build       # start everything, rebuild images
docker compose down              # stop everything
docker compose down -v           # stop everything AND wipe the Postgres/Redis volumes
docker compose run --rm migrate  # (re)run migrations without restarting everything
docker compose logs -f auth      # tail one service's logs
```

## Repo layout

```
backend/
├── go.work                  # ties shared + both services together for local dev
├── shared/                  # JWT logic, generated gRPC code, common types — no business logic
├── services/
│   ├── gateway/              # public REST API, JWT verification, rate limiting
│   └── auth/                 # LinkedIn OIDC exchange, user records, token issuance
├── proto/                    # source-of-truth .proto files (shared/ holds the generated Go code)
├── db/migrations/            # golang-migrate SQL files, applied by the `migrate` compose service
└── docker-compose.yml
```
