package auth

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log"
	"time"

	"golang.org/x/oauth2"

	"github.com/nguyentanvinh/league-be/internal/common"
	"github.com/nguyentanvinh/league-be/internal/platform/config"
	"github.com/nguyentanvinh/league-be/internal/secret"
)

// loginTxTTL bounds how long a user may sit on Google's consent screen.
const loginTxTTL = 10 * time.Minute

type Service interface {
	StartGoogleLogin(ctx context.Context) (LoginRedirect, error)
	CompleteLogin(ctx context.Context, in CallbackInput) (LoginResult, error)
	SessionUser(ctx context.Context, sessionID string) (*User, error)
	Logout(ctx context.Context, sessionID string) error
	StartSessionCleanup(ctx context.Context)
}

type service struct {
	config     *config.Config
	google     IdentityProvider
	repo       Repository
	sealer     *secret.Box
	sessionTTL time.Duration
	idleTTL    time.Duration
}

func NewService(config *config.Config, google IdentityProvider, repo Repository, sealer *secret.Box) Service {
	return &service{
		google:     google,
		repo:       repo,
		config:     config,
		sealer:     sealer,
		sessionTTL: config.Auth.SessionTTL,
		idleTTL:    config.Auth.SessionIdleTTL,
	}
}

func (s *service) StartGoogleLogin(ctx context.Context) (LoginRedirect, error) {
	state, err := common.RandomToken()
	if err != nil {
		return LoginRedirect{}, err
	}
	nonce, err := common.RandomToken()
	if err != nil {
		return LoginRedirect{}, err
	}

	codeVerifier := oauth2.GenerateVerifier()

	oauthTx, err := sealLoginTx(s.sealer, LoginTransaction{
		State:        state,
		Nonce:        nonce,
		CodeVerifier: codeVerifier,
		ExpiresAt:    time.Now().Add(loginTxTTL),
	})
	if err != nil {
		return LoginRedirect{}, err
	}

	return LoginRedirect{
		AuthURL: s.google.AuthCodeURL(state, nonce, codeVerifier),
		OAuthTx: oauthTx,
	}, nil
}

// CompleteLogin refuses a forged or expired oauth_tx cookie, or a mismatched
// state, before Google is contacted. Replay is stopped by Google's codes being
// one-time, not by us.
func (s *service) CompleteLogin(ctx context.Context, in CallbackInput) (LoginResult, error) {
	if in.OAuthTx == "" {
		return LoginResult{}, fmt.Errorf("%w: no oauth_tx cookie", ErrLoginRejected)
	}

	tx, err := openLoginTx(s.sealer, in.OAuthTx)
	if err != nil {
		return LoginResult{}, err
	}

	if subtle.ConstantTimeCompare([]byte(in.State), []byte(tx.State)) != 1 {
		return LoginResult{}, fmt.Errorf("%w: state mismatch", ErrLoginRejected)
	}

	claims, err := s.google.Identify(ctx, in.Code, tx.CodeVerifier, tx.Nonce)
	if err != nil {
		return LoginResult{}, err
	}

	user, isNewUser, err := s.repo.UpsertGoogleIdentity(ctx, GoogleIdentityParams{
		Subject:       claims.Subject,
		Email:         claims.Email,
		EmailVerified: claims.EmailVerified,
		Name:          claims.Name,
		AvatarURL:     claims.Picture,
		HD:            claims.HD,
	})
	if err != nil {
		return LoginResult{}, err
	}

	// A fresh id every login defeats session fixation, where an attacker plants
	// a known session id before sign-in.
	sessionID, err := common.RandomToken()
	if err != nil {
		return LoginResult{}, err
	}

	if err := s.repo.CreateSession(ctx, Session{ID: sessionID, UserID: user.ID}); err != nil {
		return LoginResult{}, err
	}

	return LoginResult{SessionID: sessionID, IsNewUser: isNewUser, User: user}, nil
}

func (s *service) SessionUser(ctx context.Context, sessionID string) (*User, error) {
	now := time.Now()

	session, err := s.repo.GetSession(ctx, sessionID, now.Add(-s.sessionTTL), now.Add(-s.idleTTL))
	if err != nil {
		return nil, err
	}

	user, err := s.repo.GetUserByID(ctx, session.UserID)
	if err != nil {
		return nil, err
	}

	// Extend the idle window on every read: the write is cheap, and skipping it
	// would let an active session expire mid-use.
	if err := s.repo.UpdateSessionLastSeenAt(ctx, sessionID); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *service) Logout(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return nil
	}
	return s.repo.DeleteSession(ctx, sessionID)
}

const cleanupInterval = 24 * time.Hour

func (s *service) StartSessionCleanup(ctx context.Context) {
	sweep := func() {
		now := time.Now()
		if deleted, err := s.repo.DeleteExpiredSessions(ctx, now.Add(-s.sessionTTL), now.Add(-s.idleTTL)); err != nil {
			log.Printf("auth: session cleanup: %v", err)
		} else if deleted > 0 {
			log.Printf("auth: cleanup removed %d expired sessions", deleted)
		}
	}

	go func() {
		sweep()

		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sweep()
			}
		}
	}()
}
