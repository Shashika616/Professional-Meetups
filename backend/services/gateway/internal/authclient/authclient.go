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
}

// Client is the gateway's view of the auth service.
type Client interface {
	CompleteLinkedInOnboarding(ctx context.Context, authorizationCode, pkceVerifier, redirectURI string) (Session, error)
	RefreshSession(ctx context.Context, refreshToken string) (Session, error)
	// RevokeSession is idempotent — revoking an already-revoked or unknown
	// token is not an error.
	RevokeSession(ctx context.Context, refreshToken string) error
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

func (c *grpcClient) CompleteLinkedInOnboarding(
	ctx context.Context, authorizationCode, pkceVerifier, redirectURI string,
) (Session, error) {
	resp, err := c.client.CompleteLinkedInOnboarding(ctx, &authv1.CompleteLinkedInOnboardingRequest{
		AuthorizationCode: authorizationCode,
		PkceVerifier:      pkceVerifier,
		RedirectUri:       redirectURI,
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

func sessionFromProto(resp *authv1.SessionResponse) Session {
	return Session{
		UserID:                      resp.GetUserId(),
		AccessToken:                 resp.GetAccessToken(),
		RefreshToken:                resp.GetRefreshToken(),
		AccessTokenExpiresInSeconds: resp.GetAccessTokenExpiresInSeconds(),
		IsNewUser:                   resp.GetIsNewUser(),
	}
}
