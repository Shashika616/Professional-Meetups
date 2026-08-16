// Package apperror defines the sentinel errors shared across services and
// the two boundary-mapping functions (gRPC status, then HTTP status) that
// translate them at each layer. Services should wrap one of these sentinels
// via fmt.Errorf("...: %w", ErrX) rather than returning ad-hoc errors, so
// GRPCCode/HTTPStatusFromGRPC below stay the single, complete mapping —
// not five different switch statements scattered through handlers.
package apperror

import "errors"

var (
	ErrNotFound     = errors.New("not found")
	ErrInvalidInput = errors.New("invalid input")
	ErrUnauthorized = errors.New("unauthorized")
	ErrConflict     = errors.New("conflict")
	ErrInternal     = errors.New("internal error")
)
