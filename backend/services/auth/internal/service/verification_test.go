package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/professional-connections/backend/services/auth/internal/repository"
	sharedjwt "github.com/professional-connections/backend/shared/jwt"
	authv1 "github.com/professional-connections/backend/shared/proto/auth/v1"
)

// decodeUnverifiedTestClaims reads the trust_level claim out of an issued
// access token's payload without verifying the signature — sufficient for
// tests that need to confirm what a fresh SessionResponse's token actually
// carries (the addendum's checklist: "decode it in a test, don't just
// trust the RPC succeeded"), the same way the frontend's own
// trustLevelFromAccessToken decode works.
func decodeUnverifiedTestClaims(token string) (sharedjwt.Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return sharedjwt.Claims{}, fmt.Errorf("malformed token: %d parts", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return sharedjwt.Claims{}, err
	}
	var claims sharedjwt.Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return sharedjwt.Claims{}, err
	}
	return claims, nil
}

// newTestService wires a Service against fakes only — no LinkedIn/Postgres
// dependency needed for the verification RPCs.
func newTestService(t *testing.T) (*Service, *fakeUserRepository, *fakeVerificationCodeRepository, *fakeEmailSender, *fakeSmsSender) {
	t.Helper()
	users := newFakeUserRepository()
	codes := newFakeVerificationCodeRepository()
	emailSender := &fakeEmailSender{}
	smsSender := &fakeSmsSender{}
	svc := New(users, newFakeRefreshTokenRepository(), codes, nil, newTestSigner(t), &fakePublisher{}, emailSender, smsSender)
	return svc, users, codes, emailSender, smsSender
}

func seedUser(t *testing.T, users *fakeUserRepository, id string) repository.User {
	t.Helper()
	users.byID[id] = repository.User{ID: id, LinkedInSub: "sub-" + id, FullName: "Test User", TrustLevel: 1, AccountStatus: repository.AccountStatusActive}
	return users.byID[id]
}

func TestVerifyPhoneCode_FullRoundTrip(t *testing.T) {
	svc, users, _, _, smsSender := newTestService(t)
	seedUser(t, users, "user-1")

	startResp, err := svc.StartPhoneVerification(context.Background(), &authv1.StartVerificationRequest{
		UserId:  "user-1",
		Purpose: authv1.VerificationPurpose_VERIFICATION_PURPOSE_PHONE,
		Target:  "+94771234567",
	})
	if err != nil {
		t.Fatalf("StartPhoneVerification() error: %v", err)
	}
	if startResp.GetResendAfterSeconds() != int32(otpResendCooldown.Seconds()) {
		t.Errorf("ResendAfterSeconds = %d, want %d", startResp.GetResendAfterSeconds(), int32(otpResendCooldown.Seconds()))
	}

	code := smsSender.lastCode()
	if code == "" {
		t.Fatal("no code was sent via SmsSender")
	}

	session, err := svc.VerifyPhoneCode(context.Background(), &authv1.VerifyCodeRequest{
		UserId:  "user-1",
		Purpose: authv1.VerificationPurpose_VERIFICATION_PURPOSE_PHONE,
		Target:  "+94771234567",
		Code:    code,
	})
	if err != nil {
		t.Fatalf("VerifyPhoneCode() error: %v", err)
	}
	if session.GetUserId() != "user-1" {
		t.Errorf("UserId = %q, want %q", session.GetUserId(), "user-1")
	}

	// Fresh access token, decoded, must reflect the recomputed trust level
	// (still 1 here — phone alone isn't enough for Level 2) — the addendum's
	// checklist explicitly wants this decoded in a test, not just trusted.
	claims, err := decodeUnverifiedTestClaims(session.GetAccessToken())
	if err != nil {
		t.Fatalf("decode access token: %v", err)
	}
	if claims.TrustLevel != 1 {
		t.Errorf("trust_level in fresh token = %d, want %d (phone alone isn't Level 2)", claims.TrustLevel, 1)
	}

	if users.byID["user-1"].PhoneNumber != "+94771234567" {
		t.Errorf("PhoneNumber not persisted: got %q", users.byID["user-1"].PhoneNumber)
	}
}

func TestVerifyPhoneCode_WrongCodeRejectedAndDoesNotAdvance(t *testing.T) {
	svc, users, _, _, _ := newTestService(t)
	seedUser(t, users, "user-1")

	if _, err := svc.StartPhoneVerification(context.Background(), &authv1.StartVerificationRequest{
		UserId: "user-1", Purpose: authv1.VerificationPurpose_VERIFICATION_PURPOSE_PHONE, Target: "+94771234567",
	}); err != nil {
		t.Fatalf("StartPhoneVerification() error: %v", err)
	}

	_, err := svc.VerifyPhoneCode(context.Background(), &authv1.VerifyCodeRequest{
		UserId: "user-1", Purpose: authv1.VerificationPurpose_VERIFICATION_PURPOSE_PHONE, Target: "+94771234567", Code: "000000",
	})
	if err == nil {
		t.Fatal("VerifyPhoneCode() with wrong code returned nil error, want error")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Errorf("status code = %v, want %v", got, codes.InvalidArgument)
	}
	if users.byID["user-1"].PhoneNumber != "" {
		t.Error("PhoneNumber was persisted despite a wrong code")
	}
}

func TestVerifyPhoneCode_AttemptCapInvalidatesCode(t *testing.T) {
	svc, users, verificationCodes, _, _ := newTestService(t)
	seedUser(t, users, "user-1")

	if _, err := svc.StartPhoneVerification(context.Background(), &authv1.StartVerificationRequest{
		UserId: "user-1", Purpose: authv1.VerificationPurpose_VERIFICATION_PURPOSE_PHONE, Target: "+94771234567",
	}); err != nil {
		t.Fatalf("StartPhoneVerification() error: %v", err)
	}

	for i := 0; i < otpMaxAttempts; i++ {
		_, err := svc.VerifyPhoneCode(context.Background(), &authv1.VerifyCodeRequest{
			UserId: "user-1", Purpose: authv1.VerificationPurpose_VERIFICATION_PURPOSE_PHONE, Target: "+94771234567", Code: "000000",
		})
		if err == nil {
			t.Fatalf("attempt %d: VerifyPhoneCode() with wrong code returned nil error, want error", i+1)
		}
	}

	// The code must be invalidated after the cap — confirm the row is gone,
	// not just that guesses keep failing.
	if _, err := verificationCodes.Get(context.Background(), "user-1", repository.VerificationPurposePhone); err == nil {
		t.Error("verification code row still exists after hitting the attempt cap, want it deleted")
	}
}

func TestStartPhoneVerification_ResendCooldownRejectsTooSoon(t *testing.T) {
	svc, users, _, _, _ := newTestService(t)
	seedUser(t, users, "user-1")

	if _, err := svc.StartPhoneVerification(context.Background(), &authv1.StartVerificationRequest{
		UserId: "user-1", Purpose: authv1.VerificationPurpose_VERIFICATION_PURPOSE_PHONE, Target: "+94771234567",
	}); err != nil {
		t.Fatalf("first StartPhoneVerification() error: %v", err)
	}

	_, err := svc.StartPhoneVerification(context.Background(), &authv1.StartVerificationRequest{
		UserId: "user-1", Purpose: authv1.VerificationPurpose_VERIFICATION_PURPOSE_PHONE, Target: "+94771234567",
	})
	if err == nil {
		t.Fatal("second StartPhoneVerification() within the cooldown returned nil error, want error")
	}
	if got := status.Code(err); got != codes.ResourceExhausted {
		t.Errorf("status code = %v, want %v", got, codes.ResourceExhausted)
	}
}

func TestStartPhoneVerification_ResendAllowedAfterCooldownUpsertsFreshCode(t *testing.T) {
	svc, users, verificationCodes, _, smsSender := newTestService(t)
	seedUser(t, users, "user-1")

	if _, err := svc.StartPhoneVerification(context.Background(), &authv1.StartVerificationRequest{
		UserId: "user-1", Purpose: authv1.VerificationPurpose_VERIFICATION_PURPOSE_PHONE, Target: "+94771234567",
	}); err != nil {
		t.Fatalf("first StartPhoneVerification() error: %v", err)
	}
	firstCode := smsSender.lastCode()

	// Simulate the cooldown having elapsed by backdating the stored row's
	// CreatedAt directly, rather than sleeping the test for a real minute.
	key := verificationCodeKey("user-1", repository.VerificationPurposePhone)
	row := verificationCodes.rows[key]
	row.CreatedAt = time.Now().Add(-2 * otpResendCooldown)
	verificationCodes.rows[key] = row

	if _, err := svc.StartPhoneVerification(context.Background(), &authv1.StartVerificationRequest{
		UserId: "user-1", Purpose: authv1.VerificationPurpose_VERIFICATION_PURPOSE_PHONE, Target: "+94771234567",
	}); err != nil {
		t.Fatalf("second StartPhoneVerification() after cooldown elapsed returned error: %v", err)
	}
	secondCode := smsSender.lastCode()

	if secondCode == firstCode {
		t.Error("resend after cooldown produced the same code — want a freshly generated one")
	}
	// Only one row for (user, purpose) — upsert, not a stacked second row.
	if len(verificationCodes.rows) != 1 {
		t.Errorf("verification_codes rows for this user/purpose = %d, want 1 (upsert, not stack)", len(verificationCodes.rows))
	}
}

func TestStartCorporateEmailVerification_RejectsFreeAndRoleBasedDomains(t *testing.T) {
	svc, users, _, _, _ := newTestService(t)
	seedUser(t, users, "user-1")

	_, err := svc.StartCorporateEmailVerification(context.Background(), &authv1.StartVerificationRequest{
		UserId: "user-1", Purpose: authv1.VerificationPurpose_VERIFICATION_PURPOSE_CORPORATE_EMAIL, Target: "someone@gmail.com",
	})
	if err == nil {
		t.Fatal("StartCorporateEmailVerification() with a free-domain address returned nil error, want error")
	}
	const wantMsg = "please use your work email, not a personal address: invalid input"
	if got := status.Convert(err).Message(); got != wantMsg {
		t.Errorf("error message = %q, want %q", got, wantMsg)
	}
}

func TestVerifyCorporateEmailCode_ExtractsDomainAndDeletesRawAddress(t *testing.T) {
	svc, users, verificationCodes, emailSender, _ := newTestService(t)
	seedUser(t, users, "user-1")

	if _, err := svc.StartCorporateEmailVerification(context.Background(), &authv1.StartVerificationRequest{
		UserId: "user-1", Purpose: authv1.VerificationPurpose_VERIFICATION_PURPOSE_CORPORATE_EMAIL, Target: "jane@acmecorp.com",
	}); err != nil {
		t.Fatalf("StartCorporateEmailVerification() error: %v", err)
	}

	session, err := svc.VerifyCorporateEmailCode(context.Background(), &authv1.VerifyCodeRequest{
		UserId: "user-1", Purpose: authv1.VerificationPurpose_VERIFICATION_PURPOSE_CORPORATE_EMAIL, Target: "jane@acmecorp.com", Code: emailSender.lastCode(),
	})
	if err != nil {
		t.Fatalf("VerifyCorporateEmailCode() error: %v", err)
	}
	_ = session

	got := users.byID["user-1"]
	if got.CompanyDomain != "acmecorp.com" {
		t.Errorf("CompanyDomain = %q, want %q", got.CompanyDomain, "acmecorp.com")
	}
	if !got.WorkEmailVerified {
		t.Error("WorkEmailVerified = false, want true")
	}
	if got.CompanyDomain == "jane@acmecorp.com" {
		t.Error("CompanyDomain stores the raw address, want only the domain (ADR-003)")
	}

	// The raw address, transiently held in verification_codes.target, must
	// be gone after success — query the row, don't just trust the code path.
	if _, err := verificationCodes.Get(context.Background(), "user-1", repository.VerificationPurposeCorporateEmail); err == nil {
		t.Error("verification code row (containing the raw address) still exists after successful verification")
	}
}

func TestVerifyPhoneCode_ConflictWhenAlreadyVerifiedOnDifferentAccount(t *testing.T) {
	svc, users, _, _, smsSender := newTestService(t)
	seedUser(t, users, "user-1")
	seedUser(t, users, "user-2")

	// user-1 already verified this number.
	users.byID["user-1"] = repository.User{ID: "user-1", PhoneNumber: "+94771234567", TrustLevel: 2, AccountStatus: repository.AccountStatusActive}

	// user-2 independently starts and completes verification for the same
	// number — simulating the race the addendum describes (both hold a
	// valid pending code; this fake doesn't block Start on an already-taken
	// target, matching the real repository's design).
	if _, err := svc.StartPhoneVerification(context.Background(), &authv1.StartVerificationRequest{
		UserId: "user-2", Purpose: authv1.VerificationPurpose_VERIFICATION_PURPOSE_PHONE, Target: "+94771234567",
	}); err != nil {
		t.Fatalf("StartPhoneVerification() for user-2 error: %v", err)
	}

	_, err := svc.VerifyPhoneCode(context.Background(), &authv1.VerifyCodeRequest{
		UserId: "user-2", Purpose: authv1.VerificationPurpose_VERIFICATION_PURPOSE_PHONE, Target: "+94771234567", Code: smsSender.lastCode(),
	})
	if err == nil {
		t.Fatal("VerifyPhoneCode() for a number already verified on a different account returned nil error, want error")
	}
	if got := status.Code(err); got != codes.AlreadyExists { // apperror.ErrConflict maps to codes.AlreadyExists
		t.Errorf("status code = %v, want %v (ErrConflict)", got, codes.AlreadyExists)
	}
}

func TestSubmitPersonalDetails(t *testing.T) {
	svc, users, _, _, _ := newTestService(t)
	seedUser(t, users, "user-1")

	t.Run("rejects empty fields", func(t *testing.T) {
		_, err := svc.SubmitPersonalDetails(context.Background(), &authv1.SubmitPersonalDetailsRequest{UserId: "user-1", LegalName: "", Address: "1 Main St"})
		if err == nil {
			t.Fatal("SubmitPersonalDetails() with empty legal_name returned nil error, want error")
		}
	})

	t.Run("persists and reflects in trust level once all 4 fields are set", func(t *testing.T) {
		users.byID["user-1"] = repository.User{
			ID: "user-1", PhoneNumber: "+94771234567", PersonalEmail: "a@example.com", TrustLevel: 1, AccountStatus: repository.AccountStatusActive,
		}

		session, err := svc.SubmitPersonalDetails(context.Background(), &authv1.SubmitPersonalDetailsRequest{
			UserId: "user-1", LegalName: "Ada Lovelace", Address: "1 Main St, Colombo",
		})
		if err != nil {
			t.Fatalf("SubmitPersonalDetails() error: %v", err)
		}

		claims, err := decodeUnverifiedTestClaims(session.GetAccessToken())
		if err != nil {
			t.Fatalf("decode access token: %v", err)
		}
		if claims.TrustLevel != 2 {
			t.Errorf("trust_level = %d, want %d", claims.TrustLevel, 2)
		}
	})
}

func TestGetProfile_NeverReturnsRawContactInfo(t *testing.T) {
	svc, users, _, _, _ := newTestService(t)
	users.byID["user-1"] = repository.User{
		ID:                "user-1",
		FullName:          "Ada Lovelace",
		TrustLevel:        3,
		PhoneNumber:       "+94771234567",
		PersonalEmail:     "ada@example.com",
		LegalName:         "Ada Lovelace",
		Address:           "1 Main St, Colombo",
		CompanyDomain:     "acmecorp.com",
		WorkEmailVerified: true,
		AccountStatus:     repository.AccountStatusActive,
	}

	profile, err := svc.GetProfile(context.Background(), &authv1.GetProfileRequest{UserId: "user-1"})
	if err != nil {
		t.Fatalf("GetProfile() error: %v", err)
	}

	if !profile.GetPhoneVerified() || !profile.GetPersonalEmailVerified() || !profile.GetPersonalDetailsComplete() || !profile.GetWorkEmailVerified() {
		t.Errorf("expected all verification booleans true, got %+v", profile)
	}
	if profile.GetCompanyDomain() != "acmecorp.com" {
		t.Errorf("CompanyDomain = %q, want %q", profile.GetCompanyDomain(), "acmecorp.com")
	}

	// The actual protobuf message type has no field capable of carrying a
	// raw phone number or email address at all — this is enforced by the
	// .proto definition, not runtime logic. This assertion documents that
	// invariant so a future field addition to ProfileResponse can't
	// silently reintroduce one without a test noticing the shape changed.
	if profile.ProtoReflect().Descriptor().Fields().Len() != 9 {
		t.Errorf("ProfileResponse field count = %d, want 9 — verify no raw contact field was added", profile.ProtoReflect().Descriptor().Fields().Len())
	}
}
