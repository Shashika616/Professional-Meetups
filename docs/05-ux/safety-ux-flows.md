# Safety UX Flows

Moment-by-moment flows for the riskiest parts of the product. Backed by [Safety Features Catalog](../03-architecture/safety-features-catalog.md) and [Trust & Safety Architecture](../03-architecture/trust-and-safety-architecture.md).

## Safety Gate — before a real-world meetup

1. Both users match.
2. Both select intent.
3. Both choose a public meeting category.
4. Both confirm they want to meet.
5. App suggests safe public locations ([Safety Features Catalog](../03-architecture/safety-features-catalog.md) § Meeting safety — Smart pickup recommendations logic applies here too).
6. Both users accept the meeting.
7. App creates a meetup record.
8. Safety checklist appears.
9. Optional live-location sharing activates.
10. Check-in timer starts.
11. Both users confirm after the meeting; if one doesn't confirm, the app proactively checks whether they're safe.

## Onboarding safety messaging

At account creation: *"We verify professionals, but verification does not guarantee safety. Always meet in public, never share OTPs, and report suspicious behavior."*

## Pre-meetup safety messaging

*"Choose a public place, tell someone where you are going, and use the check-in feature."*

## Pre-ride safety messaging (Phase 2+, see [Roadmap](../06-roadmap/roadmap.md))

*"Confirm driver details, share your trip with a trusted contact, and use the SOS button if needed."*

## Suspicious-message intervention

When automated risk detection flags a message: *"For your safety, do not send money or share personal codes."* — shown in-context, not just logged silently.

## Off-platform migration nudge

When a user attempts to share external contact details early: *"For your safety, keep conversations inside the app until you are confident the person is genuine."* Framed as safety guidance, not punishment — see [Safety Features Catalog](../03-architecture/safety-features-catalog.md) § Making off-platform migration harder.

## SOS / emergency

Persistent access to: call local emergency services, share live location with trusted contacts, notify the app safety team, trigger a fake incoming call, show safety instructions, instantly cancel the meetup, stop all location sharing with the matched user. Emergency numbers resolve by device location, not a hardcoded country.

## Post-meetup feedback

Ask: did the meeting happen? did you feel safe? was the profile accurate? would you meet again? do you want to report this person? Feeds trust score; never surfaced publicly in detail.

## Verified-badge disclaimer (shown on tap)

*"Verified does not guarantee good intentions. Always meet in public and trust your instincts."*

## Related

[Safety Features Catalog](../03-architecture/safety-features-catalog.md) · [Trust & Safety Architecture](../03-architecture/trust-and-safety-architecture.md) · [Domain Model](../02-domain/domain-model.md) § Meetup
