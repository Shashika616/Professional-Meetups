package service

import (
	"fmt"

	"github.com/professional-connections/backend/services/meetup/internal/repository"
	"github.com/professional-connections/backend/shared/apperror"
)

// requiredTrustLevel mirrors the frontend's IntentType.requiredTrustLevel
// exactly (ADR-013 § 2, frontend/lib/core/models/intent_type.dart) — noted
// explicitly as intentional duplication, same as the intent_type Postgres
// enum. ride_share/dating need Level 4; everything else needs Level 2 (was
// Level 1 before ADR-013 — hosting/joining a real-world meetup with a
// stranger is "the real floor for matching and messaging with strangers"
// per ADR-006).
func requiredTrustLevel(intent repository.Intent) int {
	switch intent {
	case repository.IntentRideShare, repository.IntentDating:
		return 4
	default:
		return 2
	}
}

// checkTrustLevel is the server-side trust gate, non-negotiable per
// backend/meetup-scheduling-PLAN.md Step B — called from CreateMeetup and
// RequestToJoin regardless of what the client's own UI already gates on.
// callerTrustLevel comes from the JWT's trust_level claim, threaded through
// by the gateway (never a value the client sets directly) — the claim can
// only be stale *low* (a user's trust level only ever increases, never
// decreases, so an unrefreshed token under-grants at worst, never
// over-grants), which is the safe direction for a gate to be wrong in.
func checkTrustLevel(intent repository.Intent, callerTrustLevel int) error {
	required := requiredTrustLevel(intent)
	if callerTrustLevel < required {
		return fmt.Errorf(
			"meetup: intent %q requires trust level %d, caller has %d: %w",
			intent, required, callerTrustLevel, apperror.ErrForbidden,
		)
	}
	return nil
}
