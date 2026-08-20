package service

import (
	"errors"
	"testing"

	"github.com/professional-connections/backend/services/meetup/internal/repository"
	"github.com/professional-connections/backend/shared/apperror"
)

func TestRequiredTrustLevel(t *testing.T) {
	tests := []struct {
		intent repository.Intent
		want   int
	}{
		{repository.IntentCoffee, 2},
		{repository.IntentLunch, 2},
		{repository.IntentNetworking, 2},
		{repository.IntentMentorship, 2},
		{repository.IntentRideShare, 4},
		{repository.IntentDating, 4},
	}
	for _, tt := range tests {
		t.Run(string(tt.intent), func(t *testing.T) {
			if got := requiredTrustLevel(tt.intent); got != tt.want {
				t.Errorf("requiredTrustLevel(%s) = %d, want %d", tt.intent, got, tt.want)
			}
		})
	}
}

func TestCheckTrustLevel(t *testing.T) {
	tests := []struct {
		name       string
		intent     repository.Intent
		trustLevel int
		wantErr    bool
	}{
		{"level 2 unlocks coffee", repository.IntentCoffee, 2, false},
		{"level 3 unlocks coffee (above floor)", repository.IntentCoffee, 3, false},
		{"level 1 does not unlock coffee", repository.IntentCoffee, 1, true},
		{"level 0 does not unlock coffee", repository.IntentCoffee, 0, true},
		{"level 4 unlocks ride_share", repository.IntentRideShare, 4, false},
		{"level 2 does not unlock ride_share", repository.IntentRideShare, 2, true},
		{"level 3 does not unlock dating", repository.IntentDating, 3, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkTrustLevel(tt.intent, tt.trustLevel)
			if tt.wantErr && err == nil {
				t.Fatalf("checkTrustLevel(%s, %d) = nil, want an error", tt.intent, tt.trustLevel)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("checkTrustLevel(%s, %d) = %v, want nil", tt.intent, tt.trustLevel, err)
			}
			if tt.wantErr && !errors.Is(err, apperror.ErrForbidden) {
				t.Errorf("checkTrustLevel(%s, %d) error = %v, want it to wrap apperror.ErrForbidden", tt.intent, tt.trustLevel, err)
			}
		})
	}
}
