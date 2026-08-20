// Package authclient wraps the generated gRPC client for the auth service.
// Handlers call this package's typed methods, never the generated stub
// directly, so gRPC-specific error handling and connection management live
// in one place.
package authclient

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"

	authv1 "github.com/professional-connections/backend/shared/proto/auth/v1"
)

// connectTimeout bounds how long New waits for the initial connection to
// the auth service before failing fast — a service that can't reach its
// dependencies should crash at startup, not accept traffic and fail every
// request.
const connectTimeout = 5 * time.Second

// Session is this package's own representation of an issued session,
// decoupled from the generated protobuf type — mirrors
// internal/repository's pattern of not leaking generated types across a
// boundary.
type Session struct {
	UserID                      string
	AccessToken                 string
	RefreshToken                string
	AccessTokenExpiresInSeconds int64
	IsNewUser                   bool
	FullName                    string
	ProfilePhotoURL             string
}

// Profile is this package's own representation of GetProfile's response —
// booleans/derived fields only, never a raw phone number or email address
// (backend/PLAN.md's Level 2/3 addendum, Step E).
type Profile struct {
	UserID                  string
	FullName                string
	ProfilePhotoURL         string
	TrustLevel              int
	PhoneVerified           bool
	PersonalEmailVerified   bool
	PersonalDetailsComplete bool
	CompanyDomain           string
	WorkEmailVerified       bool
}

// Client is the gateway's view of the auth service. Every Level 2/3
// verification method (backend/PLAN.md's matching addendum) takes userID
// explicitly, set by the caller (internal/handlers) from the verified
// JWT — never a client-supplied value.
type Client interface {
	// CompleteFederatedSignup creates a Level 0 account, or logs in if this
	// (provider, subject) already has one (ADR-014). provider is the REST
	// wire string ("apple"/"google") — mapped to the proto enum here, not
	// in internal/handlers, so that mapping lives in exactly one place.
	// Unauthenticated at the gateway (Register) — this is how a caller
	// gets their first token via Apple/Google.
	CompleteFederatedSignup(ctx context.Context, provider, idToken string, ageConfirmedOver18 bool) (Session, error)
	// CompleteLinkedInOnboarding creates a Level 1 account directly via
	// LinkedIn, or logs in if this linkedin_sub already has one — unchanged
	// in behavior from ADR-011, unauthenticated at the gateway.
	CompleteLinkedInOnboarding(ctx context.Context, authorizationCode, redirectURI string, ageConfirmedOver18 bool) (Session, error)
	// LinkIdentity links an identity to the caller's already-authenticated
	// account (ADR-014's Profile "Connect LinkedIn" flow, or a future "add
	// Apple/Google as backup sign-in") — userID set by the caller from the
	// verified JWT, never client-supplied. provider is the REST wire string
	// ("apple" | "google" | "linkedin"); idToken is used for apple/google,
	// authorizationCode/redirectURI for linkedin — the caller passes only
	// the pair relevant to provider, the other is ignored server-side.
	LinkIdentity(ctx context.Context, userID, provider, idToken, authorizationCode, redirectURI string) (Session, error)
	// StartEmailSignup sends an OTP to email as the first step of the
	// email+password signup flow (ADR-014 decision #2). Unauthenticated.
	StartEmailSignup(ctx context.Context, email string) (resendAfterSeconds int32, err error)
	// CompleteEmailSignup verifies the OTP sent by StartEmailSignup and
	// creates (or, per SignUpOrRecoverWithEmail, recovers) an account.
	// Unauthenticated.
	CompleteEmailSignup(ctx context.Context, email, code, password string, ageConfirmedOver18 bool) (Session, error)
	// LoginWithPassword signs in with email+password. Unauthenticated.
	LoginWithPassword(ctx context.Context, email, password string) (Session, error)
	RefreshSession(ctx context.Context, refreshToken string) (Session, error)
	// RevokeSession is idempotent — revoking an already-revoked or unknown
	// token is not an error.
	RevokeSession(ctx context.Context, refreshToken string) error

	StartPhoneVerification(ctx context.Context, userID, phoneNumber string) (resendAfterSeconds int32, err error)
	VerifyPhoneCode(ctx context.Context, userID, phoneNumber, code string) (Session, error)
	StartPersonalEmailVerification(ctx context.Context, userID, email string) (resendAfterSeconds int32, err error)
	VerifyPersonalEmailCode(ctx context.Context, userID, email, code string) (Session, error)
	SubmitPersonalDetails(ctx context.Context, userID, legalName, address string) (Session, error)
	StartCorporateEmailVerification(ctx context.Context, userID, email string) (resendAfterSeconds int32, err error)
	VerifyCorporateEmailCode(ctx context.Context, userID, email, code string) (Session, error)
	GetProfile(ctx context.Context, userID string) (Profile, error)

	Close() error
}

type grpcClient struct {
	conn   *grpc.ClientConn
	client authv1.AuthServiceClient
}

// New connects to the auth service at addr (e.g. "auth:9090"), blocking
// until the connection is ready or connectTimeout elapses.
func New(addr string) (Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("authclient: create grpc client for %s: %w", addr, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()

	conn.Connect()
	for {
		state := conn.GetState()
		if state == connectivity.Ready {
			break
		}
		if !conn.WaitForStateChange(ctx, state) {
			_ = conn.Close()
			return nil, fmt.Errorf("authclient: connect to auth service at %s: %w", addr, ctx.Err())
		}
	}

	return &grpcClient{conn: conn, client: authv1.NewAuthServiceClient(conn)}, nil
}

func (c *grpcClient) Close() error {
	return c.conn.Close()
}

// providerFromWire maps the REST wire string to the proto enum — the one
// place that mapping happens, not duplicated in internal/handlers too.
func providerFromWire(provider string) (authv1.IdentityProviderProto, error) {
	switch provider {
	case "apple":
		return authv1.IdentityProviderProto_IDENTITY_PROVIDER_APPLE, nil
	case "google":
		return authv1.IdentityProviderProto_IDENTITY_PROVIDER_GOOGLE, nil
	case "linkedin":
		return authv1.IdentityProviderProto_IDENTITY_PROVIDER_LINKEDIN, nil
	default:
		return authv1.IdentityProviderProto_IDENTITY_PROVIDER_UNSPECIFIED, fmt.Errorf("authclient: unknown identity provider %q", provider)
	}
}

func (c *grpcClient) CompleteFederatedSignup(
	ctx context.Context, provider, idToken string, ageConfirmedOver18 bool,
) (Session, error) {
	providerProto, err := providerFromWire(provider)
	if err != nil {
		return Session{}, err
	}

	resp, err := c.client.CompleteFederatedSignup(ctx, &authv1.CompleteFederatedSignupRequest{
		Provider:            providerProto,
		IdToken:             idToken,
		AgeConfirmedOver_18: ageConfirmedOver18,
	})
	if err != nil {
		return Session{}, err
	}
	return sessionFromProto(resp), nil
}

func (c *grpcClient) CompleteLinkedInOnboarding(
	ctx context.Context, authorizationCode, redirectURI string, ageConfirmedOver18 bool,
) (Session, error) {
	resp, err := c.client.CompleteLinkedInOnboarding(ctx, &authv1.CompleteLinkedInOnboardingRequest{
		AuthorizationCode:   authorizationCode,
		RedirectUri:         redirectURI,
		AgeConfirmedOver_18: ageConfirmedOver18,
	})
	if err != nil {
		return Session{}, err
	}
	return sessionFromProto(resp), nil
}

func (c *grpcClient) LinkIdentity(
	ctx context.Context, userID, provider, idToken, authorizationCode, redirectURI string,
) (Session, error) {
	providerProto, err := providerFromWire(provider)
	if err != nil {
		return Session{}, err
	}

	resp, err := c.client.LinkIdentity(ctx, &authv1.LinkIdentityRequest{
		UserId:            userID,
		Provider:          providerProto,
		IdToken:           idToken,
		AuthorizationCode: authorizationCode,
		RedirectUri:       redirectURI,
	})
	if err != nil {
		return Session{}, err
	}
	return sessionFromProto(resp), nil
}

func (c *grpcClient) StartEmailSignup(ctx context.Context, email string) (int32, error) {
	resp, err := c.client.StartEmailSignup(ctx, &authv1.StartVerificationRequest{
		Purpose: authv1.VerificationPurpose_VERIFICATION_PURPOSE_EMAIL_SIGNUP,
		Target:  email,
	})
	if err != nil {
		return 0, err
	}
	return resp.GetResendAfterSeconds(), nil
}

func (c *grpcClient) CompleteEmailSignup(
	ctx context.Context, email, code, password string, ageConfirmedOver18 bool,
) (Session, error) {
	resp, err := c.client.CompleteEmailSignup(ctx, &authv1.CompleteEmailSignupRequest{
		Email:               email,
		Code:                code,
		Password:            password,
		AgeConfirmedOver_18: ageConfirmedOver18,
	})
	if err != nil {
		return Session{}, err
	}
	return sessionFromProto(resp), nil
}

func (c *grpcClient) LoginWithPassword(ctx context.Context, email, password string) (Session, error) {
	resp, err := c.client.LoginWithPassword(ctx, &authv1.LoginWithPasswordRequest{
		Email:    email,
		Password: password,
	})
	if err != nil {
		return Session{}, err
	}
	return sessionFromProto(resp), nil
}

func (c *grpcClient) RefreshSession(ctx context.Context, refreshToken string) (Session, error) {
	resp, err := c.client.RefreshSession(ctx, &authv1.RefreshSessionRequest{RefreshToken: refreshToken})
	if err != nil {
		return Session{}, err
	}
	return sessionFromProto(resp), nil
}

func (c *grpcClient) RevokeSession(ctx context.Context, refreshToken string) error {
	_, err := c.client.RevokeSession(ctx, &authv1.RevokeSessionRequest{RefreshToken: refreshToken})
	return err
}

func (c *grpcClient) StartPhoneVerification(ctx context.Context, userID, phoneNumber string) (int32, error) {
	resp, err := c.client.StartPhoneVerification(ctx, &authv1.StartVerificationRequest{
		UserId:  userID,
		Purpose: authv1.VerificationPurpose_VERIFICATION_PURPOSE_PHONE,
		Target:  phoneNumber,
	})
	if err != nil {
		return 0, err
	}
	return resp.GetResendAfterSeconds(), nil
}

func (c *grpcClient) VerifyPhoneCode(ctx context.Context, userID, phoneNumber, code string) (Session, error) {
	resp, err := c.client.VerifyPhoneCode(ctx, &authv1.VerifyCodeRequest{
		UserId:  userID,
		Purpose: authv1.VerificationPurpose_VERIFICATION_PURPOSE_PHONE,
		Target:  phoneNumber,
		Code:    code,
	})
	if err != nil {
		return Session{}, err
	}
	return sessionFromProto(resp), nil
}

func (c *grpcClient) StartPersonalEmailVerification(ctx context.Context, userID, email string) (int32, error) {
	resp, err := c.client.StartPersonalEmailVerification(ctx, &authv1.StartVerificationRequest{
		UserId:  userID,
		Purpose: authv1.VerificationPurpose_VERIFICATION_PURPOSE_PERSONAL_EMAIL,
		Target:  email,
	})
	if err != nil {
		return 0, err
	}
	return resp.GetResendAfterSeconds(), nil
}

func (c *grpcClient) VerifyPersonalEmailCode(ctx context.Context, userID, email, code string) (Session, error) {
	resp, err := c.client.VerifyPersonalEmailCode(ctx, &authv1.VerifyCodeRequest{
		UserId:  userID,
		Purpose: authv1.VerificationPurpose_VERIFICATION_PURPOSE_PERSONAL_EMAIL,
		Target:  email,
		Code:    code,
	})
	if err != nil {
		return Session{}, err
	}
	return sessionFromProto(resp), nil
}

func (c *grpcClient) SubmitPersonalDetails(ctx context.Context, userID, legalName, address string) (Session, error) {
	resp, err := c.client.SubmitPersonalDetails(ctx, &authv1.SubmitPersonalDetailsRequest{
		UserId:    userID,
		LegalName: legalName,
		Address:   address,
	})
	if err != nil {
		return Session{}, err
	}
	return sessionFromProto(resp), nil
}

func (c *grpcClient) StartCorporateEmailVerification(ctx context.Context, userID, email string) (int32, error) {
	resp, err := c.client.StartCorporateEmailVerification(ctx, &authv1.StartVerificationRequest{
		UserId:  userID,
		Purpose: authv1.VerificationPurpose_VERIFICATION_PURPOSE_CORPORATE_EMAIL,
		Target:  email,
	})
	if err != nil {
		return 0, err
	}
	return resp.GetResendAfterSeconds(), nil
}

func (c *grpcClient) VerifyCorporateEmailCode(ctx context.Context, userID, email, code string) (Session, error) {
	resp, err := c.client.VerifyCorporateEmailCode(ctx, &authv1.VerifyCodeRequest{
		UserId:  userID,
		Purpose: authv1.VerificationPurpose_VERIFICATION_PURPOSE_CORPORATE_EMAIL,
		Target:  email,
		Code:    code,
	})
	if err != nil {
		return Session{}, err
	}
	return sessionFromProto(resp), nil
}

func (c *grpcClient) GetProfile(ctx context.Context, userID string) (Profile, error) {
	resp, err := c.client.GetProfile(ctx, &authv1.GetProfileRequest{UserId: userID})
	if err != nil {
		return Profile{}, err
	}
	return Profile{
		UserID:                  resp.GetUserId(),
		FullName:                resp.GetFullName(),
		ProfilePhotoURL:         resp.GetProfilePhotoUrl(),
		TrustLevel:              int(resp.GetTrustLevel()),
		PhoneVerified:           resp.GetPhoneVerified(),
		PersonalEmailVerified:   resp.GetPersonalEmailVerified(),
		PersonalDetailsComplete: resp.GetPersonalDetailsComplete(),
		CompanyDomain:           resp.GetCompanyDomain(),
		WorkEmailVerified:       resp.GetWorkEmailVerified(),
	}, nil
}

func sessionFromProto(resp *authv1.SessionResponse) Session {
	return Session{
		UserID:                      resp.GetUserId(),
		AccessToken:                 resp.GetAccessToken(),
		RefreshToken:                resp.GetRefreshToken(),
		AccessTokenExpiresInSeconds: resp.GetAccessTokenExpiresInSeconds(),
		IsNewUser:                   resp.GetIsNewUser(),
		FullName:                    resp.GetFullName(),
		ProfilePhotoURL:             resp.GetProfilePhotoUrl(),
	}
}
