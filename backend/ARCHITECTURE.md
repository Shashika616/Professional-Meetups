# Backend Architecture Map

What each file/folder is for, and why it's shaped this way. `README.md` is
"how to run it"; `PLAN.md` was "how to build the first slice"; this is "what
exists and what it does" — the one to read when something breaks and you
need to know where to look. Business/product *why* lives in the vault's
`04 - Decisions` (ADRs) and `docs/` — this file only covers code shape.

## Top-level layout

```
backend/
├── go.work                  # ties shared/ + all three services together for local dev only
├── docker-compose.yml        # local Postgres, Redis, Pub/Sub emulator, migrate, gateway, auth, meetup
├── .env / .env.example       # local config — LinkedIn/Twilio/Resend/Firebase creds, DB/Redis addresses, GCP project
├── secrets/                  # local-only JWT RSA keypair, gitignored, never committed
├── .golangci.yml              # lint rules enforced in CI (errcheck, staticcheck, govet, ineffassign)
├── buf.yaml / buf.gen.yaml    # config for generating Go code from proto/
├── proto/auth/v1/auth.proto   # source-of-truth gRPC contract for auth (hand-edited)
├── proto/meetup/v1/meetup.proto # source-of-truth gRPC contract for meetup (hand-edited, ADR-013)
├── db/migrations/             # golang-migrate SQL files, applied by the `migrate` compose service
├── shared/                    # code shared by every service — no business logic
└── services/
    ├── gateway/               # public REST API — the only thing the internet can reach
    ├── auth/                  # LinkedIn OIDC exchange, user records, token issuance, Level 2/3 verification
    └── meetup/                # host-initiated meetup scheduling, join requests, Safety Gate, push notifications (ADR-013)
```

**The dependency direction is one-way**: `services/*` import `shared/`;
`shared/` never imports from a service. If you ever see `shared/` importing
something from `services/gateway` or `services/auth`, that's a bug in the
boundary, not a new feature.

## `shared/` — code every service needs, no business logic

- **`shared/proto/auth/v1/`** — `auth.pb.go` / `auth_grpc.pb.go`, generated
  from `proto/auth/v1/auth.proto` via `buf` (Step 2 in PLAN.md). **Never
  hand-edit these** — edit the `.proto` and regenerate. Committed to git so a
  fresh clone builds without needing `buf` installed.
- **`shared/proto/meetup/v1/`** — same deal for `proto/meetup/v1/meetup.proto`
  (ADR-013, `backend/meetup-scheduling-PLAN.md`).
- **`shared/jwt/`** — `claims.go` (the `Claims` struct: `UserID`,
  `TrustLevel`, standard `exp`/`iat`/`iss`/`sub`), `signer.go` (loads the
  RSA *private* key, signs tokens — **only `services/auth` ever constructs
  a `Signer`**, per ADR-009), `verifier.go` (loads only the *public* key,
  verifies a token — this is what the gateway uses). If a token won't
  verify, this is the first place to check; if a token can't be signed at
  all, check `secrets/jwt_private.pem` exists and is readable.
- **`shared/apperror/`** — `apperror.go` defines seven sentinel errors
  (`ErrNotFound`, `ErrInvalidInput`, `ErrUnauthorized`, `ErrConflict`,
  `ErrInternal`, `ErrRateLimited` — added for the Level 2/3 verification
  addendum's server-enforced resend cooldown, distinct from the gateway's
  IP-based rate limiter below but mapping to the same HTTP 429 — and
  `ErrForbidden`, added for the meetup-scheduling slice's trust-level gate
  and host/requester ownership checks — "authenticated but not permitted,"
  the 401-vs-403 distinction from `ErrUnauthorized`); `grpc.go` maps each to
  a gRPC status code (and redacts `ErrInternal`
  to a fixed generic message before it reaches a client — the real error is
  logged server-side via `slog.Default()` instead); `http.go` maps gRPC
  status codes to HTTP status codes. This is the **only** place that
  translation happens — if an API is returning the wrong HTTP status for an
  error, this is where to look, not in the individual handler.
- **`shared/logging/`** — `logger.go` (JSON `slog` logger, matches Cloud
  Logging's expected format), `requestid.go` (generates/propagates a
  request ID), `httpmiddleware.go` / `grpcinterceptor.go` (attach that
  request ID to every log line for a request, one for each protocol). If
  logs from a request aren't correlating, this is the layer responsible.

## `services/auth/` — LinkedIn OIDC exchange, user records, token issuance

```
services/auth/
├── cmd/server/main.go        # wiring only — start here to see how everything connects
└── internal/
    ├── config/                # env var loading, fails fast on missing required vars (Twilio/Resend creds are the one deliberate exception — see below)
    ├── linkedin/               # this backend's half of the LinkedIn OIDC exchange (no PKCE — see PLAN.md's 2026-08-17 correction)
    ├── email/                  # EmailSender interface: LoggingEmailSender (default) + ResendEmailSender
    ├── sms/                    # SmsSender interface: LoggingSmsSender (default) + TwilioSmsSender
    ├── repository/             # Postgres access — interfaces first, sqlc-generated queries underneath
    │   └── sqlcgen/             # generated by sqlc from repository/queries/*.sql — never hand-edit
    ├── service/                # business logic — implements the generated gRPC AuthServiceServer
    │   ├── trustlevel.go        # computeTrustLevel — the one place trust-level rules live
    │   ├── otp.go                # shared OTP generation/hashing/verification + free/role-based domain rejection
    │   └── verification.go       # the 8 Level 2/3 RPCs (Start*/Verify*/SubmitPersonalDetails/GetProfile)
    └── events/                 # publishes the user.onboarded Pub/Sub event
```

- **`cmd/server/main.go`** — the entrypoint. Loads config, connects to
  Postgres (`pgxpool`), loads the JWT signer, connects to Pub/Sub, builds a
  `linkedin.Client`, wires everything into a `service.Service`, starts the
  gRPC server, and handles `SIGTERM` for graceful shutdown. If the auth
  service won't start at all, the error will point at one of these steps in
  order — read top to bottom.
- **`internal/linkedin/client.go`** — `ExchangeCode` (trades an
  authorization code for a LinkedIn access token — no PKCE verifier;
  LinkedIn's Sign In with LinkedIn / OpenID Connect product rejects the
  exchange when one is present, see `PLAN.md`'s 2026-08-17 correction) and
  `FetchUserInfo` (calls LinkedIn's userinfo endpoint for `sub`/`name`/
  `picture`). The LinkedIn access token is discarded immediately after — it
  never gets passed anywhere else. If LinkedIn login is failing, this is
  where the actual HTTP calls to LinkedIn happen.
- **`internal/email/`** — `email.go` defines the `EmailSender` interface
  (`SendVerificationCode(ctx, to, code, purpose)`); `logging.go`
  (`LoggingEmailSender`, the default — logs only the code and purpose,
  never the raw address, so local dev/every automated test works without a
  real Resend account) and `resend.go` (`ResendEmailSender`, real delivery
  via `resend-go`). `cmd/server/main.go` picks the real sender only when
  `RESEND_API_KEY`/`RESEND_FROM_EMAIL` are both non-empty.
- **`internal/sms/`** — same shape as `internal/email/` for phone:
  `sms.go` (`SmsSender` interface), `logging.go` (`LoggingSmsSender`,
  default), `twilio.go` (`TwilioSmsSender` — hand-rolled `net/http` against
  Twilio's plain Programmable Messaging API, deliberately not the Twilio
  Verify SDK; see ADR-012's 2026-08-17 correction for why). Real sender
  wired only when all three `TWILIO_*` vars are non-empty.
- **`internal/repository/`** — `repository.go` defines the `UserRepository`,
  `RefreshTokenRepository`, and `VerificationCodeRepository` **interfaces**
  (what `internal/service/` actually depends on); `users_postgres.go` /
  `refresh_tokens_postgres.go` / `verification_codes_postgres.go` are the
  Postgres implementations, built on `sqlcgen/` (generated,
  parameterized-by-construction queries — this is what makes SQL injection
  structurally impossible here, not developer discipline). `convert.go`
  translates between `sqlcgen`'s generated row types and the domain types
  in `repository.go`. If a query is wrong, check `repository/queries/*.sql`
  first (the source you actually edit) — `sqlcgen/` regenerates from it.
  `UpdatePhoneNumber`/`UpdatePersonalEmail` are what surface migration
  0002's `UNIQUE` constraint as `apperror.ErrConflict` on a 23505 — the
  two-users-race-for-the-same-target case resolves here, not in
  `internal/service`.
- **`internal/service/service.go`** — the original Level 1a business logic:
  `CompleteLinkedInOnboarding` (exchange code → look up or create user →
  issue session → publish event if new), `RefreshSession` (rotate a
  refresh token, rejecting replay/expired tokens as a possible theft
  signal per ADR-009), `RevokeSession` (idempotent logout).
- **`internal/service/trustlevel.go`** — `computeTrustLevel(user) int`, the
  **one** place trust-level rules live (mirrors `apperror`'s single-mapping-
  table discipline). Level 3 requires all four Level 2 fields *and*
  `WorkEmailVerified` — there is deliberately no partial credit for 1-of-4
  Level 2 fields. Called after every mutation in `verification.go`, before
  the row is persisted or a fresh token issued.
- **`internal/service/otp.go`** — the one OTP mechanism shared by phone/
  personal-email/corporate-email (ADR-012's 2026-08-17 correction):
  `generateOTP`/`hashOTP`/`otpMatches` (6-digit, `crypto/rand`, SHA-256
  hash, 10-minute expiry, 5-attempt cap — all as named constants at the top
  of this file), plus `isRejectedCorporateEmail`/`domainFromEmail`
  (Verification Model § 5's free-domain/role-based-address lists, MVP-
  scoped per ADR-012).
- **`internal/service/verification.go`** — the 8 Level 2/3 RPCs
  (`Start*Verification`/`Verify*Code`/`SubmitPersonalDetails`/
  `GetProfile`). `startVerificationRPC` and `verifyAndConsumeCode` are the
  shared implementations every `Start*`/`Verify*` RPC wraps — one code
  path per direction, purpose-specific only in which `Sender` it dispatches
  to and which `User` field it persists. Every verification-completing RPC
  reissues a fresh access/refresh pair via the same `issueSession` helper
  `service.go` already uses, so a client never shows a stale trust level
  after an action that just changed it.
- **`internal/events/events.go`** — publishes `user.onboarded` to Pub/Sub
  with a minimal payload (`user_id`, `trust_level`, `occurred_at` only —
  no name/photo/PII).

## `services/meetup/` — host-initiated meetup scheduling, join requests, Safety Gate (ADR-013)

```
services/meetup/
├── cmd/server/main.go        # wiring only — same shape as services/auth's
└── internal/
    ├── config/                # env var loading (FIREBASE_SERVICE_ACCOUNT_JSON is the one optional var — see below)
    ├── repository/             # Postgres access — interfaces first, sqlc-generated queries underneath
    │   └── sqlcgen/             # generated by sqlc from repository/queries/*.sql — never hand-edit
    ├── service/                # business logic — implements the generated gRPC MeetupServiceServer
    │   ├── trustgate.go          # requiredTrustLevel/checkTrustLevel — the one place the per-intent trust floor lives
    │   ├── requests.go           # RequestToJoin/WithdrawRequest/RespondToRequest
    │   ├── safety.go             # the Safety Gate sub-flow (checklist/live-location/check-in/feedback)
    │   ├── convert.go            # domain <-> protobuf conversion (Intent/MeetupStatus/MeetupRequestStatus enums)
    │   └── cursor.go             # opaque keyset-pagination cursor encode/decode for ListOpenMeetups
    ├── events/                 # publishes meetup.request.{created,accepted,rejected} to Pub/Sub
    └── notifications/          # Sender interface: LoggingPushSender (default) + FCMPushSender
```

- **Reads (but never writes) the `users` table `services/auth` owns** —
  `internal/repository`'s meetup/meetup-request queries `JOIN users` to
  return host/requester display info (name, photo, trust level) alongside
  a meetup or request, rather than making a second gRPC call to
  `services/auth` for it. This is a deliberate boundary-crossing read
  (both services already share one physical Postgres database — see
  `db/migrations/0003_meetups.up.sql`'s own comment), not an ownership
  violation: `services/meetup` has no `UPDATE`/`INSERT` against `users`
  anywhere in its query set.
- **`internal/repository/meetups.sql` / `meetup_requests.sql`** — the
  `GetMeetupByID`/`ListOpenMeetupsByIntent*` queries use a **plain `LEFT
  JOIN`**, not `LEFT JOIN LATERAL` or a scalar subquery, for the viewer's
  `my_request_status` column — confirmed by generating all three forms and
  inspecting sqlc's output: only the plain `LEFT JOIN` form produced the
  correctly-nullable `NullMeetupRequestStatus` Go type. The other two
  produced a non-nullable type that would panic scanning a real `NULL` row
  (a viewer who's never requested to join — the common browsing case). If a
  future query needs the same "nullable column via join" shape, mirror this
  form, not the other two.
- **`internal/repository/meetup_requests_postgres.go`'s `Accept`** — the
  capacity-check-and-auto-reject transaction (backend/meetup-scheduling-
  PLAN.md Step B): `SELECT ... FOR UPDATE` locks the meetup row for the
  duration of the transaction, so two near-simultaneous `RespondToRequest`
  accept calls against the same meetup can never both succeed past
  capacity — the second blocks until the first commits, then re-reads the
  now-current status under the same lock. Covered by a real-Postgres
  integration test (`internal/service/integration_test.go`'s
  `TestCapacityRace_Integration`) that fires two genuinely concurrent
  goroutines, not two sequential calls — a mocked-repository unit test
  cannot exercise this, only real Postgres row locking can.
- **`internal/service/trustgate.go`** — `requiredTrustLevel(intent)`
  mirrors the frontend's `IntentType.requiredTrustLevel` exactly (ADR-013
  § 2: `ride_share`/`dating` need Level 4, everything else needs Level 2).
  `checkTrustLevel` is called from `CreateMeetup` and `RequestToJoin`
  against the caller's trust level as threaded through by the gateway from
  the JWT claim (`middleware.TrustLevelFromContext`) — never a
  client-supplied value. Browsing (`ListOpenMeetups`/`GetMeetup`) has no
  gate at all, per ADR-013 § 2's low-friction-browsing goal.
- **`internal/notifications/`** — `notifications.go` defines the `Sender`
  interface (`SendPushNotification(ctx, userID, title, body, data)` — takes
  a user, not a device token; resolving "which device(s) does this user
  have" is the sender's own job via `DeviceTokenRepository`); `logging.go`
  (`LoggingPushSender`, the default — logs the payload instead of calling
  FCM, no raw device token anywhere in that log line); `fcm.go`
  (`FCMPushSender`, real delivery via the FCM HTTP v1 API, authenticated
  with a Firebase service account via `cloud.google.com/go/auth/
  credentials` + `httptransport` — deliberately not the deprecated
  `golang.org/x/oauth2/google.CredentialsFromJSON`/`WithParams` functions,
  see `fcm.go`'s own comment). `cmd/server/main.go` picks the real sender
  only when `FIREBASE_SERVICE_ACCOUNT_JSON` is non-empty.
- **`internal/events/events.go`** — three topics
  (`meetup-request-created`/`-accepted`/`-rejected`), each payload minimal
  (IDs and the fact of the transition only — no meetup location/timing or
  requester name; the notification-dispatch side looks up display text
  itself rather than the event payload carrying it).
- A **new table not in ADR-013's own text**: `device_tokens` (migration
  0003) — FCM push can't be sent without somewhere to register a
  recipient's device, flagged and added during the build
  (`backend/meetup-scheduling-PLAN.md` Step B). Registered via
  `RegisterDeviceToken`, upserted by token (not by user) — a token
  identifies one physical device install.

## `services/gateway/` — public REST API, the only public entry point

```
services/gateway/
├── cmd/server/main.go        # wiring — start here too
└── internal/
    ├── config/                 # env var loading
    ├── handlers/                # REST endpoints — translates JSON ↔ the auth/meetup services' gRPC contracts
    ├── middleware/               # rate limiting, request logging, panic recovery, JWT auth
    ├── authclient/               # typed Go wrapper around auth's generated gRPC client
    └── meetupclient/             # typed Go wrapper around meetup's generated gRPC client (ADR-013)
```

- **`cmd/server/main.go`** — loads config, connects to the auth service and
  the meetup service (both gRPC) and Redis, constructs a `jwt.Verifier`
  (see `middleware/auth.go` below), registers routes, then layers
  middleware around the whole mux in this order (outermost first):
  `Recover` → `HTTPMiddleware` (request-ID)
  → `RequestLogging` → `RateLimit` → the actual routes. If you're debugging
  "why didn't my middleware run," check this ordering — it determines what
  happens before what.
- **`internal/handlers/handlers.go`** — the three original Level 1a REST
  endpoints (`POST /v1/auth/linkedin/callback`, `/v1/auth/refresh`,
  `/v1/auth/logout`). Each handler: decode JSON → call `authclient` →
  translate the gRPC response/error back to JSON/HTTP. `sessionResponse`
  (the shape `linkedin/callback`/`refresh`/every verification-completing
  route return) is `user_id`, `access_token`, `refresh_token`,
  `expires_in`, `is_new_user`, `full_name`, `profile_photo_url`.
  `trust_level` is deliberately **not** here — it's only in the signed
  access token's JWT claims (`shared/jwt.Claims`), read via a client-side
  decode rather than a response field.
- **`internal/handlers/verification.go`** — the Level 2/3 REST endpoints:
  `POST /v1/verification/{phone,personal-email,corporate-email}/{start,verify}`,
  `POST /v1/verification/personal-details`, `GET /v1/users/me`. Every one
  of these is registered in `handlers.go`'s `Register` wrapped individually
  in `h.requireAuth` (see `middleware/auth.go`) — not applied globally in
  `cmd/server/main.go`, so the original three routes stay unauthenticated
  at the gateway layer exactly as before. `profileResponse` never carries a
  raw phone number or email address — booleans/derived fields only.
- **`internal/handlers/meetups.go`** — the `/v1/meetups/*` REST endpoints
  (ADR-013, `backend/meetup-scheduling-PLAN.md` Step D): create/browse/get/
  cancel a meetup, list "my meetups", list/request/withdraw/respond-to join
  requests, register a device token, the four Safety Gate endpoints. All
  wrapped in `h.requireAuth` like `verification.go`'s routes.
  `host_user_id`/`requester_id` and `host_trust_level`/`requester_trust_level`
  are always set from `middleware.UserIDFromContext`/
  `middleware.TrustLevelFromContext` — never trusted from the request body.
- **`internal/authclient/authclient.go`** — the only file that talks gRPC
  to the auth service directly; handlers never touch the generated gRPC
  stub themselves, so gRPC-specific error handling lives in exactly one
  place. Every Level 2/3 method takes `userID` as an explicit parameter,
  set by the handler from the verified JWT — never a client-supplied value.
- **`internal/meetupclient/meetupclient.go`** — same pattern as
  `authclient.go`, for the meetup service. Its own `Meetup`/`MeetupRequest`/
  `SafetyState` types translate proto enums (`Intent`/`MeetupStatus`/
  `MeetupRequestStatus`) to plain lowercase strings once here (e.g.
  `"coffee"`, `"open"`, `"pending"`) — `internal/handlers/meetups.go` never
  touches a proto enum directly.
- **`internal/middleware/ratelimit.go`** — Redis-backed fixed-window
  counter, 20 requests/minute per (IP, route), applied to every route on
  the mux (originally just `/v1/auth/*`; the Level 2/3 verification routes
  and the `/v1/meetups/*` routes both share the same limiter by virtue of
  being registered on the same mux, not a second limiter each). **Fails
  open** — if Redis is down, requests are allowed through rather than the
  entire auth surface going down; rate limiting is defense in depth, not
  the only line of defense. If legitimate requests are getting 429'd, this
  file + Redis's current counters are where to check.
- **`internal/middleware/recover.go`** — turns a panic in any handler into
  a logged 500 instead of crashing the process.
- **`internal/middleware/auth.go`** — `Auth(verifier)`: verifies the
  `Authorization: Bearer <access_token>` header, rejects with 401 if
  missing/invalid/expired, attaches the verified `user_id` **and
  `trust_level`** to the request context (`UserIDFromContext`/
  `TrustLevelFromContext` — the latter added for the meetup-scheduling
  slice's trust gate; the claim can only be stale *low*, since trust level
  only ever increases, which is the safe direction for a gate to be wrong
  in). **This is the first thing in this service's history that verifies a
  JWT at all** — confirmed by reading the pre-addendum code, not assumed;
  `JWT_PUBLIC_KEY_PATH` was already forward-provisioned in
  `docker-compose.yml` but never read anywhere until this. Applied
  per-route at handler-registration time (`internal/handlers.Register`),
  not globally.

## Where to look when something breaks

- **Service won't start** → `cmd/server/main.go` for that service, read
  top-to-bottom; the error names which dependency (Postgres, Redis,
  Pub/Sub, JWT key, LinkedIn config) failed to initialize.
- **A request returns the wrong data** → `internal/handlers` (gateway) or
  `internal/service` (auth/meetup) — that's where the actual logic lives,
  not the repository or transport layers.
- **A request returns the wrong HTTP status** → `shared/apperror` — the
  single mapping table, not the handler.
- **Wrong data in the database** → `internal/repository/queries/*.sql` and
  `internal/repository/*_postgres.go` — regenerate `sqlcgen/` after
  editing a `.sql` file (`sqlc generate` from `services/auth/`).
- **A token won't verify / won't sign** → `shared/jwt` and
  `secrets/jwt_private.pem` / `jwt_public.pem`.
- **LinkedIn login itself is failing** → `internal/linkedin/client.go`
  (auth service) — this is the only place that talks to LinkedIn's actual
  API.
- **A `/v1/verification/*` or `/v1/users/me` call returns 401 even with a
  token that looks fine** → `services/gateway/internal/middleware/auth.go`
  — check the header is literally `Authorization: Bearer <token>` and that
  `JWT_PUBLIC_KEY_PATH` on the gateway points at the same keypair
  `JWT_PRIVATE_KEY_PATH` on the auth service signed with.
  `services/auth/internal/service/trustlevel.go` for whether a trust level
  looks wrong after a verification succeeds — decode the returned access
  token, don't assume the RPC updated it correctly.
- **A code never arrives during local testing** → check `cmd/server/main.go`
  (auth service)'s startup log line ("verification email/SMS delivery:
  ...") to confirm which sender is actually wired; with `LoggingSmsSender`/
  `LoggingEmailSender` (the default until `TWILIO_*`/`RESEND_*` are filled
  in), the code is in the auth service's own stdout, not an inbox/phone.
- **A push notification never arrives** → same pattern, meetup service:
  check `cmd/server/main.go`'s startup log line ("push notification
  delivery: ...") — with `LoggingPushSender` (the default until
  `FIREBASE_SERVICE_ACCOUNT_JSON` is filled in), the payload is in the
  meetup service's own stdout, not a real push. If it's `FCMPushSender` and
  still not arriving, check `device_tokens` has a row for the recipient
  (`RegisterDeviceToken` must have been called from that device first).
- **`CreateMeetup`/`RequestToJoin` rejects with 403 unexpectedly** →
  `services/meetup/internal/service/trustgate.go`'s `requiredTrustLevel`
  table, and whether the caller's JWT trust_level claim is actually current
  (decode it — a stale-but-still-valid token from before their most recent
  verification would under-report, not over-report, per the gateway auth
  middleware's own doc comment).
- **Two hosts/requests near capacity produce an inconsistent accepted
  count** → `services/meetup/internal/repository/meetup_requests_postgres.go`'s
  `Accept` — check the `SELECT ... FOR UPDATE` is still there and still
  the first statement inside the transaction; `TestCapacityRace_Integration`
  in the same package's `integration_test.go` is the regression test for
  this exact class of bug.

## Related

Vault: `04 - Decisions` (ADR-008 platform, ADR-009 auth, ADR-011 the
LinkedIn onboarding slice, ADR-012 Level 2/3 verification, ADR-013 meetup
scheduling) for *why*; `docs/03-architecture/system-architecture.md` for
the platform-level shape; `backend/PLAN.md` and
`backend/meetup-scheduling-PLAN.md` for how each slice was built;
`backend/README.md` for how to run it locally.
