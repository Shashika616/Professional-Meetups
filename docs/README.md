# Professional Meetups — Knowledge Base

This vault is the **product and architecture knowledge base** for the Professional Connections Platform. It is the source of truth that both Cowork (product/architecture thinking) and Claude Code (implementation) should read before making decisions. See [Cowork Operating Charter](00-project/cowork-operating-charter.md) for how the two are meant to work together, and [CLAUDE.md Template (For Code Repo)](00-project/claude-md-template-for-code-repo.md) for what to paste into the actual codebase once it exists.

## How this vault is organized

- **[Vision](00-project/vision.md)** — what we're building and why, in one page.
- **[Project State](00-project/project-state.md)** — what phase we're in right now, what's decided, what's open. Update this often; everything else changes slowly.
- **01 - Product** — [Requirements](01-product/requirements.md), [Personas](01-product/personas.md), [Competitive Landscape](01-product/competitive-landscape.md), [Revenue Model](01-product/revenue-model.md). What we're building, for whom.
- **02 - Domain** — [Domain Model](02-domain/domain-model.md), [Trust Levels](02-domain/trust-levels.md), [Verification Model](02-domain/verification-model.md). What the business concepts actually mean.
- **03 - Architecture** — [System Architecture](03-architecture/system-architecture.md), [Threat Model](03-architecture/threat-model.md), [Trust & Safety Architecture](03-architecture/trust-and-safety-architecture.md), [Safety Features Catalog](03-architecture/safety-features-catalog.md), [Privacy & Anti-Abuse Controls](03-architecture/privacy-and-anti-abuse-controls.md), [Operations & Incident Response](03-architecture/operations-and-incident-response.md). How we're building it, and how we defend it.
- **04 - Decisions** — [ADR-001](04-decisions/adr-001-verified-real-world-professional-networking.md) through [ADR-006](04-decisions/adr-006-progressive-linkedin-first-trust-onboarding.md). Why we chose what we chose, so nobody re-litigates it three weeks later.
- **05 - UX** — [Safety UX Flows](05-ux/safety-ux-flows.md). The moment-by-moment flows for the riskiest parts of the product.
- **06 - Roadmap** — [Roadmap](06-roadmap/roadmap.md). What's next, phased by risk.
- **07 - Research** — [Sri Lanka PDPA - Work Email Verification](07-research/sri-lanka-pdpa-work-email-verification.md), [Sources & Citations](07-research/sources-and-citations.md). External facts backing the above.

## Authority model

1. **You (Shashika)** make final product and business calls.
2. **Cowork** (this assistant, in this vault) does product thinking, architecture, and business-rule analysis, and keeps these documents current.
3. **Claude Code** implements against these documents. It should read them before major work, never silently overrule a decision recorded in `04 - Decisions`, and flag — not resolve — anything that conflicts with them.
4. **Claude Code's own auto-memory** is working memory (local build quirks, "we tried X, it broke because Y"). It is disposable and machine-local — it is never where business truth lives.

When Claude Code discovers something that invalidates a document here (e.g. "the domain model assumed synchronous state, but we need event-driven"), that comes back here as a proposed decision, not a silent code-side rewrite.

## Vault vs. code repo

This vault lives outside the git repository. Once the codebase exists, the canonical copies of `requirements.md`, `architecture.md`, `domain.md`, and ADRs should also live in `docs/` inside the repo (so Claude Code and git history track them precisely), with this vault either mirroring them or becoming the primary source you edit and periodically sync into `docs/`. Until that repo exists, this vault *is* the source of truth.
