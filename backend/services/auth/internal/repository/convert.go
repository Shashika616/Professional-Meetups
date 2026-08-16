package repository

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
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

// textOrNull converts a plain Go string to a nullable Postgres text column,
// treating "" as NULL.
func textOrNull(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

// timestamptzOrZero converts a Postgres timestamptz to time.Time. Only used
// for columns that are NOT NULL in the schema (issued_at, expires_at,
// created_at, updated_at), where sqlc still generates pgtype.Timestamptz
// rather than time.Time.
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

// uuidPtrOrNil converts a nullable Postgres uuid column to *string.
func uuidPtrOrNil(id pgtype.UUID) *string {
	if !id.Valid {
		return nil
	}
	u := uuid.UUID(id.Bytes)
	s := u.String()
	return &s
}

// pgtypeUUID converts a uuid.UUID to a non-null pgtype.UUID for use as a
// query parameter on a nullable uuid column.
func pgtypeUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}
