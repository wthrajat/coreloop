package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"coreloop/backend/internal/config"
	"coreloop/backend/internal/ids"
	"coreloop/backend/internal/securehash"
	"coreloop/backend/internal/store"
)

const (
	SessionCookieName = "coreloop_session"
	CSRFCookieName    = "coreloop_csrf"
	SessionLifetime   = 30 * 24 * time.Hour
)

type Service struct {
	store  *store.Store
	oidc   *OIDCClient
	config config.Config
	now    func() time.Time
}

type LoginResult struct {
	User          store.User
	SessionToken  string
	CSRFToken     string
	SessionExpiry time.Time
	Created       bool
	ReturnPath    string
}

func NewService(dataStore *store.Store, configuration config.Config, client *OIDCClient) *Service {
	if client == nil {
		client = NewOIDCClient(configuration.TelegramClientID, configuration.TelegramClientSecret,
			configuration.AppOrigin+"/api/app/auth/callback", nil)
	}
	return &Service{store: dataStore, oidc: client, config: configuration, now: time.Now}
}

func (service *Service) Start(ctx context.Context, inviteToken, returnPath string) (string, error) {
	if service.config.TelegramClientID == "" || service.config.TelegramClientSecret == "" {
		return "", errors.New("Telegram login is not configured")
	}
	inviteID := ""
	if inviteToken != "" {
		invite, err := service.store.ResolveInvite(ctx, securehash.Keyed(inviteToken, service.config.SessionSecret), service.now())
		if err != nil {
			return "", errors.New("invite is invalid, expired, or already used")
		}
		inviteID = invite.ID
	}
	if returnPath == "" || !strings.HasPrefix(returnPath, "/") || strings.HasPrefix(returnPath, "//") {
		returnPath = "/onboarding"
	}
	state, err := ids.Token(32)
	if err != nil {
		return "", err
	}
	verifier, err := ids.Token(48)
	if err != nil {
		return "", err
	}
	nonce, err := ids.Token(24)
	if err != nil {
		return "", err
	}
	flowID, err := ids.New("oidc")
	if err != nil {
		return "", err
	}
	flow := store.AuthFlow{ID: flowID, InviteID: inviteID, CodeVerifier: verifier, Nonce: nonce,
		ReturnPath: returnPath, ExpiresAt: service.now().Add(10 * time.Minute)}
	if err := service.store.CreateAuthFlow(ctx, flow, securehash.SHA256(state)); err != nil {
		return "", fmt.Errorf("create login flow: %w", err)
	}
	return service.oidc.AuthorizationURL(state, nonce, PKCEChallenge(verifier)), nil
}

func (service *Service) Callback(ctx context.Context, code, stateValue string) (LoginResult, error) {
	if code == "" || stateValue == "" {
		return LoginResult{}, errors.New("Telegram login did not return code and state")
	}
	flow, err := service.store.ConsumeAuthFlow(ctx, securehash.SHA256(stateValue), service.now())
	if err != nil {
		return LoginResult{}, errors.New("login state is invalid, expired, or already used")
	}
	claims, err := service.oidc.Exchange(ctx, code, flow.CodeVerifier, flow.Nonce)
	if err != nil {
		return LoginResult{}, err
	}
	if !NumericSubject(claims.Subject) {
		return LoginResult{}, errors.New("Telegram identity subject is invalid")
	}
	user, created, err := service.store.UpsertUserFromTelegram(ctx, store.Identity{
		Subject: claims.Subject, DisplayName: claims.Name, Username: claims.PreferredUsername, AvatarURL: claims.Picture,
	}, flow.InviteID, service.now())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return LoginResult{}, errors.New("this Telegram account has not been invited")
		}
		return LoginResult{}, err
	}
	sessionToken, err := ids.Token(32)
	if err != nil {
		return LoginResult{}, err
	}
	csrfToken, err := ids.Token(24)
	if err != nil {
		return LoginResult{}, err
	}
	expires := service.now().Add(SessionLifetime)
	_, err = service.store.CreateSession(ctx, user.ID,
		securehash.Keyed(sessionToken, service.config.SessionSecret),
		securehash.Keyed(csrfToken, service.config.SessionSecret), expires)
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{User: user, SessionToken: sessionToken, CSRFToken: csrfToken,
		SessionExpiry: expires, Created: created, ReturnPath: flow.ReturnPath}, nil
}

func (service *Service) Authenticate(ctx context.Context, token string) (store.Session, store.User, error) {
	if token == "" {
		return store.Session{}, store.User{}, sql.ErrNoRows
	}
	return service.store.SessionByToken(ctx, securehash.Keyed(token, service.config.SessionSecret), service.now())
}

func (service *Service) ValidateCSRF(session store.Session, token string) bool {
	return token != "" && securehash.Equal(session.CSRFHash, securehash.Keyed(token, service.config.SessionSecret))
}

func (service *Service) IsOwner(user store.User) bool {
	return service.config.OwnerTelegramSubject != "" && user.TelegramSubject == service.config.OwnerTelegramSubject
}

func SafeReturnPath(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed.IsAbs() || !strings.HasPrefix(parsed.Path, "/") {
		return "/overview"
	}
	return parsed.RequestURI()
}
