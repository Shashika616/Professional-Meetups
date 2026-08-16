# Revenue Model

Three tiers (BR-005 in [Requirements](requirements.md)):

## Free Tier

Basic profile, limited daily matches, standard messaging.

## Premium Tier

Unlimited matches, AI-powered recommendations, advanced filters, search by company/industry, exclusive event access.

> **Guardrail ([ADR-002](../04-decisions/adr-002-multi-level-continuous-trust-scoring.md) / [Safety Features Catalog](../03-architecture/safety-features-catalog.md) § Verification Should Be Hard to Buy):** premium must never sell trust. It cannot mean faster verification without checks, more visibility while under review, bypassing reports or trust limits, contacting users who blocked them, or seeing hidden private data. "Search by company" specifically needs its own abuse controls — see [Safety Features Catalog](../03-architecture/safety-features-catalog.md) § Prevent Company Search Abuse — before it ships.

## Enterprise Tier

B2B SaaS: companies provide employees internal networking, safe commuting, cross-team collaboration, and ESG commuting initiatives. This is also the safest growth path from a trust-and-safety standpoint — see [Safety Features Catalog](../03-architecture/safety-features-catalog.md) § Enterprise Mode.

## Related

[Requirements](requirements.md) · [Personas](personas.md) · [Trust Levels](../02-domain/trust-levels.md)
