# Vision

## One-line pitch

A trusted, real-world networking app that connects verified professionals based on shared intent, location, and real-time availability — merging networking, transportation, and social-serendipity into one context-aware, safety-first utility.

## Problem

Professionals miss opportunities to build meaningful relationships in daily life because today's tools are fragmented:

- **LinkedIn** — online-only, asynchronous, no real-world/location context.
- **Ride-sharing (Uber/PickMe)** — pure point-A-to-B transport, zero networking value from the commute.
- **Dating apps** — no professional trust or verification; unsuitable for mentorship or business networking.
- **Event platforms (Meetup)** — scheduled, large-group; misses instant one-on-one serendipity.

No platform currently connects verified professionals in everyday life based on immediate intent. See [Competitive Landscape](../01-product/competitive-landscape.md) for the detailed gap analysis.

## The core scenario

Picture a corporate employee whose work life leaves little room for a social life: they want someone to grab coffee with, hang out over drinks, or go to a concert with — not another dating-app match, not a random stranger off the street. The only way that's safe to offer is to make sure the person on the other side of the match has a real professional footprint: a LinkedIn history, ideally a corporate email, a track record of showing up. **This is the product in one sentence: give busy professionals a safe, verified way to find company for real life, not just for work.**

## This is a platform for professionals, not the general public

Every gate in this product — verification, trust levels, intent-based matching — exists because we are putting strangers in a room together, and we are responsible for their safety when we do. That responsibility is why [Trust Levels](../02-domain/trust-levels.md) exist, why matching is never open to fully unverified accounts, and why the highest-risk intents (dating, ride-sharing) are deliberately held back until the trust and safety systems are proven ([ADR-004](../04-decisions/adr-004-defer-dating-and-open-ride-sharing.md)). We are not building a general social network with a "professionals" filter bolted on; the professional-verification requirement is the safety mechanism, not a marketing label.

## Who it's for

See [Personas](../01-product/personas.md) for full detail — in short: overworked professionals who want safe company for coffee, drinks, or a concert outside of work, commuters who'd rather share a ride with a peer than sit alone, professionals newly relocated to a city who want a non-dating way to meet people, founders wanting spontaneous mentorship or brainstorming, and corporate HR teams wanting safer commuting and cross-department networking.

## What makes this different

1. **Intent-based, not profile-browsing.** Users toggle a current intent (ride sharing, coffee, lunch, drinks, a concert, mentorship, dating) rather than swiping static profiles.
2. **Verified professionals only, verified progressively.** Start with LinkedIn (federated, or a lower-trust pasted URL), then layer on personal verification, then optionally corporate email and KYC — see [Verification Model](../02-domain/verification-model.md) and [Trust Levels](../02-domain/trust-levels.md). Real matching with strangers only opens up once someone has cleared the personal-verification floor, not at signup.
3. **Safety is a first-class feature, not a settings-page afterthought.** SOS/emergency flow, live-location sharing, and trusted-contact sharing are built in from day one — see [Trust & Safety Architecture](../03-architecture/trust-and-safety-architecture.md) and [Safety Features Catalog](../03-architecture/safety-features-catalog.md).
4. **Trust is continuous, not one-time.** A dynamic trust score gates access to higher-risk intents — see [Trust Levels](../02-domain/trust-levels.md).

## Where we're launching

Sri Lanka first (Colombo), targeting a closed, invite-only / company-based beta before any public open registration. See [Roadmap](../06-roadmap/roadmap.md) and [ADR-005](../04-decisions/adr-005-invite-only-company-based-closed-launch.md) in `04 - Decisions`.

## Non-goals (for now)

- Dating mode and open ride-sharing are explicitly **deferred**, not cut — see [ADR-004](../04-decisions/adr-004-defer-dating-and-open-ride-sharing.md).
- This is not a general social network; discovery is always intent-gated and verification-gated.

## Related

[Requirements](../01-product/requirements.md) · [Domain Model](../02-domain/domain-model.md) · [System Architecture](../03-architecture/system-architecture.md) · [Project State](project-state.md)
