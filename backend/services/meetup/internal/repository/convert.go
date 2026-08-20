package repository

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/professional-connections/backend/services/meetup/internal/repository/sqlcgen"
)

// textOrEmpty converts a nullable Postgres text column to a plain Go
// string, treating NULL as "" — this repository's domain types stay
// pgtype-free so internal/service/ never needs to import pgx.
func textOrEmpty(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

// timestamptzOrZero converts a Postgres timestamptz to time.Time. Only used
// for columns that are NOT NULL in the schema, where sqlc still generates
// pgtype.Timestamptz rather than time.Time.
func timestamptzOrZero(ts pgtype.Timestamptz) time.Time {
	return ts.Time
}

func toTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// timePtrOrNil converts a nullable Postgres timestamptz to *time.Time.
func timePtrOrNil(ts pgtype.Timestamptz) *time.Time {
	if !ts.Valid {
		return nil
	}
	t := ts.Time
	return &t
}

// boolPtrOrNull converts a *bool to a nullable Postgres bool column.
func boolPtrOrNull(b *bool) pgtype.Bool {
	if b == nil {
		return pgtype.Bool{}
	}
	return pgtype.Bool{Bool: *b, Valid: true}
}

// stringPtrOrNull converts a *string to a nullable Postgres text column —
// meetup_feedback.notes (ADR-016), same optional-free-text shape as
// feltSafe/profileAccurate/wouldMeetAgain's boolean counterparts.
func stringPtrOrNull(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

// stringPtrOrNil converts a nullable Postgres text column to *string — the
// inverse of stringPtrOrNull, for read paths.
func stringPtrOrNil(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	s := t.String
	return &s
}

// requestStatusPtrOrNil converts sqlc's nullable enum wrapper to
// *MeetupRequestStatus.
func requestStatusPtrOrNil(s sqlcgen.NullMeetupRequestStatus) *MeetupRequestStatus {
	if !s.Valid {
		return nil
	}
	status := MeetupRequestStatus(s.MeetupRequestStatus)
	return &status
}

func parseUUID(id string) (uuid.UUID, error) {
	return uuid.Parse(id)
}

// numericToFloat64 converts users.rating_average (NUMERIC(3,2), NOT NULL
// DEFAULT 0) to a plain float64. Falls back to 0 on an invalid/unparseable
// value rather than propagating an error — the column always has a
// well-formed default, so this only guards against a theoretical scan
// oddity, not an expected runtime case.
func numericToFloat64(n pgtype.Numeric) float64 {
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return f.Float64
}
