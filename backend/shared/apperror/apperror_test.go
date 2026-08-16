package apperror

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestToGRPCStatus(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want codes.Code
	}{
		{"not found", fmt.Errorf("user 123: %w", ErrNotFound), codes.NotFound},
		{"invalid input", fmt.Errorf("bad email: %w", ErrInvalidInput), codes.InvalidArgument},
		{"unauthorized", fmt.Errorf("bad token: %w", ErrUnauthorized), codes.Unauthenticated},
		{"conflict", fmt.Errorf("duplicate: %w", ErrConflict), codes.AlreadyExists},
		{"internal", fmt.Errorf("db down: %w", ErrInternal), codes.Internal},
		{"unrecognized error falls back to internal", errors.New("boom"), codes.Internal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := status.Code(ToGRPCStatus(tt.err))
			if got != tt.want {
				t.Errorf("ToGRPCStatus(%v) code = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestToGRPCStatusNil(t *testing.T) {
	if err := ToGRPCStatus(nil); err != nil {
		t.Errorf("ToGRPCStatus(nil) = %v, want nil", err)
	}
}

func TestHTTPStatusFromGRPC(t *testing.T) {
	tests := []struct {
		code codes.Code
		want int
	}{
		{codes.OK, http.StatusOK},
		{codes.NotFound, http.StatusNotFound},
		{codes.InvalidArgument, http.StatusBadRequest},
		{codes.Unauthenticated, http.StatusUnauthorized},
		{codes.PermissionDenied, http.StatusForbidden},
		{codes.AlreadyExists, http.StatusConflict},
		{codes.ResourceExhausted, http.StatusTooManyRequests},
		{codes.DeadlineExceeded, http.StatusGatewayTimeout},
		{codes.Unavailable, http.StatusServiceUnavailable},
		{codes.Internal, http.StatusInternalServerError},
		{codes.Unknown, http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.code.String(), func(t *testing.T) {
			if got := HTTPStatusFromGRPC(tt.code); got != tt.want {
				t.Errorf("HTTPStatusFromGRPC(%v) = %d, want %d", tt.code, got, tt.want)
			}
		})
	}
}
