package apperror

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ToGRPCStatus maps err to a gRPC status error by matching it against the
// sentinel errors in this package via errors.Is, so a service-layer error
// wrapped as fmt.Errorf("...: %w", ErrNotFound) arrives at the caller as a
// codes.NotFound status. Anything that doesn't match a known sentinel maps
// to codes.Internal — services should wrap ErrInternal explicitly rather
// than relying on this fallback, so the mapping stays intentional.
func ToGRPCStatus(err error) error {
	if err == nil {
		return nil
	}
	return status.Error(grpcCode(err), err.Error())
}

func grpcCode(err error) codes.Code {
	switch {
	case errors.Is(err, ErrNotFound):
		return codes.NotFound
	case errors.Is(err, ErrInvalidInput):
		return codes.InvalidArgument
	case errors.Is(err, ErrUnauthorized):
		return codes.Unauthenticated
	case errors.Is(err, ErrConflict):
		return codes.AlreadyExists
	default:
		return codes.Internal
	}
}
