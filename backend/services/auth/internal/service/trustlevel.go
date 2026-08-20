package service

import "github.com/professional-connections/backend/services/auth/internal/repository"

// computeTrustLevel is the one place trust-level rules live (backend/PLAN.md's
// Level 2/3 addendum, Step H) — same "single mapping table" discipline
// shared/apperror already uses. Called after every mutation that changes a
// verification field, before the row is persisted or a fresh
// SessionResponse/JWT is issued.
//
// Level 0 (ADR-014): a federated-only account (Apple/Google, no LinkedIn
// linked) is real and persisted now, not just "mid-signup" — u.LinkedInSub
// empty is an expected, reachable case, not the impossible one this
// function's comment used to claim. LinkedIn is a hard prerequisite for
// Level 2+ (ADR-014 §4, confirmed) — a caller with every Level 2 field set
// but no LinkedIn linked still computes to 0, not 2; the four
// verification-completing RPCs in verification.go also reject outright
// before ever reaching here if LinkedIn isn't linked yet, so this is
// defense in depth, not the only enforcement point.
//
// Level 3 requires Level 2's four conditions to *all* still hold, not just
// WorkEmailVerified in isolation — a user who verified only corporate email
// while skipping phone/personal-email/personal-details computes to Level 1,
// not Level 3. There is deliberately no partial credit for 1-of-4 Level 2
// fields ([[Trust Levels]] defines Level 2 as the bundle, not four
// independent gates) — the separate *continuous* trust score is where
// partial-progress signals belong, and that system isn't part of this
// addendum.
func computeTrustLevel(u repository.User) int {
	if u.LinkedInSub == "" {
		return 0
	}
	level2 := u.PhoneNumber != "" && u.PersonalEmail != "" &&
		u.LegalName != "" && u.Address != ""
	switch {
	case level2 && u.WorkEmailVerified:
		return 3
	case level2:
		return 2
	default:
		return 1
	}
}
