# Revenue Model

Three tiers (BR-005 in [Requirements](requirements.md)):

## Free Tier

Basic profile, limited daily matches, standard messaging. When viewing another user's profile (via search or a match), sees a **limited info card** only — see § Profile visibility below.

## Premium Tier

Unlimited matches, AI-powered recommendations, advanced filters, search by company/industry, exclusive event access, and **full profile info** when viewing other users.

### Profile visibility by tier (decided 2026-08-16)

No user-managed public/private profile toggle — that's unnecessary social-media-style complexity this product doesn't need. Instead, how much of a profile you can see when you find someone (via search or a match) depends on **your own subscription tier**, not a setting the *viewed* user controls: free-tier viewers see a limited card, paid viewers see the full profile. This is a generalization of "search by company" below, not a separate system.

This is unrelated to, and doesn't replace, the existing anti-stalking controls in [Safety Features Catalog](../03-architecture/safety-features-catalog.md) (rate-limited repeated searches, hiding from specific people/company/industry, no re-contact after a block) — those govern *behavior* toward a specific user regardless of who's paying, and stay as-is.

> **Guardrail ([ADR-002](../04-decisions/adr-002-multi-level-continuous-trust-scoring.md) / [Safety Features Catalog](../03-architecture/safety-features-catalog.md) § Verification Should Be Hard to Buy):** premium must never sell trust. It cannot mean faster verification without checks, more visibility while under review, bypassing reports or trust limits, contacting users who blocked them, or seeing hidden private data. "Search by company" specifically needs its own abuse controls — see [Safety Features Catalog](../03-architecture/safety-features-catalog.md) § Prevent Company Search Abuse — before it ships.

## Enterprise Tier

B2B SaaS: companies provide employees internal networking, safe commuting, cross-team collaboration, and ESG commuting initiatives. This is also the safest growth path from a trust-and-safety standpoint — see [Safety Features Catalog](../03-architecture/safety-features-catalog.md) § Enterprise Mode.

## Related

[Requirements](requirements.md) · [Personas](personas.md) · [Trust Levels](../02-domain/trust-levels.md)
