# Feature Build Plan Template

The shape `backend/PLAN.md` used for the LinkedIn onboarding slice ([ADR-011](../04-decisions/adr-011-linkedin-onboarding-slice-design.md)),
generalized so every future backend feature — matching engine, messaging,
safety-gate service, SOS, whatever's next on [Roadmap](roadmap.md) — gets built with
the same rigor instead of ad hoc. When starting a new slice, copy this
structure into a new `backend/<feature>-PLAN.md` (or a section of one, if
several small features ship together), fill it in, and hand the whole file
to Claude Code as the task brief.

## Why this shape, not just "build X"

A plan with an explicit scope boundary and a self-review checklist is what
keeps a feature from either (a) silently growing into adjacent features that
weren't decided yet, or (b) shipping without the security/test coverage a
real prod app for strangers-meeting-strangers needs. Every section below
earns its place for one of those two reasons.

## 1. Scope boundary (state this first, always)

One paragraph: exactly what this slice includes, and — just as
important — an explicit list of adjacent things it does *not* include yet
(mirrors PLAN.md's "Explicitly not in this slice" section, but stated up
front too, not just at the end). If a design decision during the build
seems to imply scope creep into one of the excluded items, the instruction
is always: stop and flag it, don't quietly build toward it.

## 2. Prerequisites (human, not the AI)

Anything requiring a human action outside the codebase — API keys,
third-party app registration, local tool installs, generated secrets. State
these as an explicit numbered step the executing agent must *confirm*
before writing code, not assume.

## 3. Build order

Shared code first (anything the feature needs that other services might
also need later), then service-by-service. For each package/service, state:
what it's responsible for, what it must NOT do (e.g. "never persist X",
mirroring the data-minimization principle applied everywhere in this
project), and which existing pattern to follow (repository pattern +
`sqlc`, interface-first services, dependency injection via constructor —
see `backend/ARCHITECTURE.md` for what's already established).

## 4. Test strategy — the actual pyramid to build

- **Unit tests** for anything with real logic and no external dependency
  (business rules, pure functions, request/response translation).
- **Component tests against a fake/stub** for anything talking to a
  third party (LinkedIn today; payment processors, other verification
  providers, etc. later) — `httptest.Server` standing in, never the real
  external service in tests.
- **Integration test(s) against the real local dependency** (real Postgres
  via docker-compose, real Redis) for anything that would only break with
  a bad migration, a bad query, or a real concurrency issue a mock can't
  catch. At least one per new persistence-touching feature.
- **Security-relevant tests specifically**: anything the feature does that
  touches auth, PII, or money gets its own explicit test — token
  tampering/expiry rejection, injection resistance (usually free from
  `sqlc`, but confirm), rate-limit behavior, idempotency of destructive
  actions (matches this project's existing pattern: refresh-token replay
  rejection, idempotent logout/revoke).

## 5. CI update

If this feature needs a new automated check (a new lint rule, a new service
added to the Postgres-backed integration-test job, etc.), say so explicitly
— don't leave it to be silently forgotten. Keep unrelated toolchains in
separate workflow files (established already: `flutter-ci.yml` vs.
`backend-ci.yml`).

## 6. Self-review checklist (do this before calling it done)

Copy and adapt PLAN.md's Step 7 shape:
- [ ] No sensitive raw value (tokens, passwords, unverified PII) ever
      logged, stored past its needed lifetime, or included in an error
      message.
- [ ] Every new SQL query is parameterized (via `sqlc` — spot-check anyway).
- [ ] Every exported function that can fail returns `error`; panics in
      request-handling paths become controlled 500s via the existing
      recovery middleware, not crashes.
- [ ] New rate limiting / abuse controls (if applicable) are actually wired
      into the router, not just implemented and unused.
- [ ] `go vet` and `golangci-lint` clean.
- [ ] `docker compose up --build` works from a clean checkout
      (`docker compose down -v` first) — the real end-to-end proof.
- [ ] Bring the diff back to Cowork for an architecture/security pass
      before merging — same discipline as every prior slice.

## 7. Record the durable decisions separately

Anything in the plan that should outlive the specific build task (a new
security principle, a new data-retention rule, a schema shape that other
features will depend on) becomes its own ADR in `04 - Decisions` — the plan
file itself stays disposable execution detail, same split [ADR-011](../04-decisions/adr-011-linkedin-onboarding-slice-design.md) already
established relative to `backend/PLAN.md`.

## Related

[Roadmap](roadmap.md) · [Project State](../00-project/project-state.md) · [ADR-011](../04-decisions/adr-011-linkedin-onboarding-slice-design.md) in `04 - Decisions` (the first
slice this pattern was extracted from) · `backend/PLAN.md`,
`backend/ARCHITECTURE.md` in the repo
