package service

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/professional-connections/backend/services/meetup/internal/repository"
)

// Cursors are opaque to callers (ListOpenMeetupsResponse.next_cursor is
// just a string) — encoded here as base64("<unix_nanos>|<uuid>") so a
// client can't infer or forge internal ordering from the string's shape,
// and so this encoding can change later without touching the wire
// contract's type.
func encodeCursor(c *repository.Cursor) string {
	if c == nil {
		return ""
	}
	raw := fmt.Sprintf("%d|%s", c.CreatedAt.UnixNano(), c.ID)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// decodeCursor returns (nil, nil) for an empty string — the first-page
// case, not an error.
func decodeCursor(encoded string) (*repository.Cursor, error) {
	if encoded == "" {
		return nil, nil
	}

	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode cursor: %w", err)
	}

	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("malformed cursor")
	}

	nanos, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("malformed cursor timestamp: %w", err)
	}

	return &repository.Cursor{
		CreatedAt: time.Unix(0, nanos).UTC(),
		ID:        parts[1],
	}, nil
}
