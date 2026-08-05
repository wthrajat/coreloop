package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"coreloop/backend/internal/config"
	"coreloop/backend/internal/ids"
	"coreloop/backend/internal/securehash"
	"coreloop/backend/internal/store"
)

const (
	SessionCookieName      = "coreloop_session"
	CSRFCookieName         = "coreloop_csrf"
	LoginBindingCookieName = "coreloop_login_binding"
	SecureLoginBindingName = "__Host-coreloop_login_binding"
	SessionLifetime        = 30 * 24 * time.Hour
	LoginBindingLifetime   = 10 * time.Minute
)

var (
	ErrLoginNotConfigured = errors.New("Telegram login is not configured")
	ErrInvalidInvite      = errors.New("invite is invalid, expired, or already used")
)

type Service struct {
	store  *store.Store
	oidc   *OIDCClient
	config config.Config
	now    func() time.Time
}

type LoginResult struct {
	User           store.User
	SessionToken   string
	CSRFToken      string
	SessionExpiry  time.Time
	Created        bool
	ReturnPath     string
	TelegramChatID string
}

type StartResult struct {
	LoginURL            string
	BrowserBindingToken string
	ExpiresAt           time.Time
}

func NewService(dataStore *store.Store, configuration config.Config, client *OIDCClient) *Service {
	if client == nil {
		client = NewOIDCClient(configuration.TelegramClientID, configuration.TelegramClientSecret,
			configuration.AppOrigin+"/api/app/auth/callback", nil)
	}
	return &Service{store: dataStore, oidc: client, config: configuration, now: time.Now}
}

func (service *Service) Start(
	ctx context.Context,
	inviteToken string,
	returnPath string,
) (StartResult, error) {
	if service.config.TelegramClientID == "" || service.config.TelegramClientSecret == "" {
		return StartResult{}, ErrLoginNotConfigured
	}
	inviteID := ""
	if inviteToken != "" {
		invite, err := service.store.ResolveInvite(ctx, securehash.Keyed(inviteToken, service.config.SessionSecret), service.now())
		if err != nil {
			return StartResult{}, ErrInvalidInvite
		}
		inviteID = invite.ID
	}
	if returnPath == "" || !strings.HasPrefix(returnPath, "/") || strings.HasPrefix(returnPath, "//") {
		returnPath = "/onboarding"
	}
	state, err := ids.Token(32)
	if err != nil {
		return StartResult{}, err
	}
	verifier, err := ids.Token(48)
	if err != nil {
		return StartResult{}, err
	}
	nonce, err := ids.Token(24)
	if err != nil {
		return StartResult{}, err
	}
	browserBindingToken, err := ids.Token(32)
	if err != nil {
		return StartResult{}, err
	}
	flowID, err := ids.New("oidc")
	if err != nil {
		return StartResult{}, err
	}
	expiresAt := service.now().Add(LoginBindingLifetime)
	flow := store.AuthFlow{ID: flowID, InviteID: inviteID, CodeVerifier: verifier, Nonce: nonce,
		ReturnPath: returnPath, BrowserBindingHash: securehash.Keyed(
			browserBindingToken,
			service.config.SessionSecret,
		), ExpiresAt: expiresAt}
	if err := service.store.CreateAuthFlow(ctx, flow, securehash.SHA256(state)); err != nil {
		return StartResult{}, fmt.Errorf("create login flow: %w", err)
	}
	return StartResult{
		LoginURL:            service.oidc.AuthorizationURL(state, nonce, PKCEChallenge(verifier)),
		BrowserBindingToken: browserBindingToken,
		ExpiresAt:           expiresAt,
	}, nil
}

func (service *Service) Callback(
	ctx context.Context,
	code string,
	stateValue string,
	browserBindingToken string,
) (LoginResult, error) {
	if code == "" || stateValue == "" || browserBindingToken == "" {
		return LoginResult{}, errors.New("Telegram login did not return code and state")
	}
	flow, err := service.store.ConsumeAuthFlow(
		ctx,
		securehash.SHA256(stateValue),
		securehash.Keyed(browserBindingToken, service.config.SessionSecret),
		service.now(),
	)
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
	identity := identityFromClaims(claims)
	user, created, err := service.store.UpsertUserFromTelegram(ctx, identity, flow.InviteID, service.now())
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
		SessionExpiry: expires, Created: created, ReturnPath: flow.ReturnPath,
		TelegramChatID: identity.TelegramChatID}, nil
}

func identityFromClaims(claims Claims) store.Identity {
	return store.Identity{
		Subject: claims.Subject, TelegramChatID: strconv.FormatInt(claims.TelegramUserID, 10),
		DisplayName: claims.Name, Username: claims.PreferredUsername, AvatarURL: claims.Picture,
	}
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
