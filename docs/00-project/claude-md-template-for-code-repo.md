# CLAUDE.md Template (For Code Repo)

This is a ready-to-copy `CLAUDE.md` for whenever the actual git repository is created. It points Claude Code at this vault's content (exported to `docs/` inside the repo) instead of duplicating it. Copy the block below into `CLAUDE.md` at the repo root, then fill in the `[bracketed]` parts once the stack is chosen (see [Project State](project-state.md) → "Blocked / needs a decision from you").

```markdown
# Project

This is the Professional Connections Platform — a verified-professional,
intent-based, real-world networking app (Sri Lanka launch, Colombo first).

## Project Documentation

Read these before making any significant architectural or business-rule
change. They are the source of truth; this file is not.

- docs/requirements.md       — what we're building (FR/NFR/business rules)
- docs/domain.md             — what the business concepts mean
- docs/architecture.md       — system architecture
- docs/trust-and-safety.md   — trust levels, threat model, safety features
- docs/decisions/            — ADRs: why we chose what we chose
- docs/roadmap.md            — what's next, phased by risk

Do not re-derive these from first principles or "improve" them silently.
If something here looks wrong or incomplete once you're in the code,
say so explicitly and propose it as a new ADR — don't quietly implement
around it.

## Development Rules

- [stack/commands once chosen — build, test, lint, dev server]

## Architecture Rules

- Respect the trust-level gating model (docs/trust-and-safety.md) for any
  feature that touches matching, messaging, location, or ride-sharing.
- Dating mode and open ride-sharing are Phase 2/3 features — see
  docs/roadmap.md and [ADR-004](../04-decisions/adr-004-defer-dating-and-open-ride-sharing.md). Do not build them into Phase 1 scope.
- Work-email verification must not persist the raw email address — see
  [ADR-003](../04-decisions/adr-003-ephemeral-work-email-verification.md) and docs/legal/sri-lanka-pdpa.md. Store only company_domain,
  verified boolean, and verified_at (90-day expiry).

## Testing Rules

- [testing framework / coverage expectations once chosen]

## Security Rules

- Never log or persist raw work emails, government IDs, or vehicle
  documents beyond what [ADR-003](../04-decisions/adr-003-ephemeral-work-email-verification.md) and docs/trust-and-safety.md § Privacy
  permit. Encrypt sensitive documents at rest; delete after verification
  where possible.
- Any new data collection touching PII needs a one-line justification
  referencing which requirement or safety control it serves.

## Git Rules

- Commit docs and the code that implements them together where practical
  (e.g. `docs: define authentication architecture` then
  `feat: implement authentication foundation`).
```

## Also create `.claude/rules/`

As the codebase grows, split out `.claude/rules/backend.md`, `frontend.md`, `database.md`, `testing.md`, `security.md` rather than growing `CLAUDE.md`. Keep `CLAUDE.md` itself under ~200 lines.

## Related

[Cowork Operating Charter](cowork-operating-charter.md) · [Project State](project-state.md)
