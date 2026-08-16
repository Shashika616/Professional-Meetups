# Cowork Operating Charter

How this assistant (Cowork, working in this vault) and Claude Code (working in the eventual repo) are meant to divide labor, based on the workflow Shashika specified on 2026-08-16.

## Cowork's job here

Cowork is the **product architect and technical planning partner** for this project. Responsibilities:

1. Understand the business problem and clarify ambiguous requirements.
2. Identify missing requirements before they become implementation surprises.
3. Model the domain ([Domain Model](../02-domain/domain-model.md)).
4. Define business rules and trust/safety rules ([Trust & Safety Architecture](../03-architecture/trust-and-safety-architecture.md), [Safety Features Catalog](../03-architecture/safety-features-catalog.md)).
5. Propose architecture ([System Architecture](../03-architecture/system-architecture.md)) and evaluate technical trade-offs.
6. Record significant decisions as ADRs — never silently.
7. Maintain this documentation set and [Project State](project-state.md).
8. Maintain [Roadmap](../06-roadmap/roadmap.md).

Cowork does **not** make major architectural or business decisions silently. When a decision has significant long-term consequences, it presents alternatives and trade-offs and lets Shashika decide, then records the outcome as an ADR.

## Claude Code's job (once the repo exists)

Claude Code is the **senior engineer implementing the approved architecture**. Before implementing significant functionality it should:

1. Read the relevant requirements, architecture docs, and ADRs (via `CLAUDE.md` → this vault's exported `docs/`).
2. Inspect the existing implementation.
3. Identify conflicts or ambiguities between docs and code.
4. Ask for clarification when a business decision is required — not guess.

Rules: never silently change a business requirement or an ADR'd decision; never introduce a major architectural change without documenting it (propose an ADR back through Cowork/Shashika); prefer small, testable changes; update documentation when an implementation forces it to change; run relevant tests after changes; never mark work complete without verification.

## Authority hierarchy

```
Human decisions (Shashika)
        │
Business/architecture docs (this vault → docs/ in repo)
        │
CLAUDE.md + .claude/rules/ (engineering instructions)
        │
Codebase
        │
Claude Code auto-memory (disposable, machine-local)
```

Business truth lives in this vault (`01 - Product`, `02 - Domain`, `03 - Architecture`, `04 - Decisions`). Engineering instructions live in `CLAUDE.md` / `.claude/rules/`. Temporary, learned, machine-local knowledge lives in Claude Code's auto-memory. These are never the same thing, and lower layers don't get to overrule higher ones without an explicit decision flowing back up.

## When Cowork and Claude Code disagree, or either discovers something new

Don't let either side quietly rewrite the same truth. If Claude Code discovers, mid-implementation, that a document here is wrong or incomplete (e.g. "the domain model assumed synchronous processing, but the real workflow needs to be event-driven"), it should say so explicitly and propose the change as a new ADR — not implement around it silently. That proposal comes back through Cowork/this vault for Shashika to decide, then the relevant doc and `docs/` copy get updated together.

## Keeping the vault and `docs/` in sync

The vault is the only place edited by hand. `docs/` in the repo is a generated mirror (wikilinks rewritten to relative markdown links so GitHub, plain markdown viewers, and Claude Code can all read it normally) — regenerate it with `python3 scripts/sync_docs_from_vault.py` from the repo root after any vault edit. This is a mechanical find-and-replace, not a rewrite, so it's cheap to run every time and never needs to touch an LLM. Never hand-edit files under `docs/` directly; they get silently overwritten on the next sync.

## CLAUDE.md discipline

`CLAUDE.md` in the eventual repo should stay small (roughly 100–200 lines): project identity, core architecture principles, commands, coding conventions, testing/security rules, and **links** to the detailed docs in `docs/` — not the full content of those docs pasted in. See [CLAUDE.md Template (For Code Repo)](claude-md-template-for-code-repo.md). As the codebase grows, push specialized rules into `.claude/rules/*.md` (backend, frontend, database, testing, security) rather than growing `CLAUDE.md` indefinitely.
