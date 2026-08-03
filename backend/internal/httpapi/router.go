package httpapi

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"coreloop/backend/internal/apperror"
	"coreloop/backend/internal/auth"
	"coreloop/backend/internal/config"
	"coreloop/backend/internal/ids"
	"coreloop/backend/internal/jobs"
	"coreloop/backend/internal/securehash"
	"coreloop/backend/internal/store"
	"coreloop/backend/internal/telegram"
)

type Config struct {
	BuildVersion string
	Runtime      config.Config
	Store        *store.Store
	Auth         *auth.Service
	Jobs         *jobs.Service
	Telegram     *telegram.Client
}

type healthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Version string `json:"version"`
	Ready   bool   `json:"ready"`
	Message string `json:"message,omitempty"`
}
type sessionContext struct {
	Session store.Session
	User    store.User
}
type contextKey string

const sessionKey contextKey = "session"

func NewRouter(configuration Config) http.Handler {
	buildVersion := strings.TrimSpace(configuration.BuildVersion)
	if buildVersion == "" {
		buildVersion = "development"
	}
	mux := http.NewServeMux()
	health := func(w http.ResponseWriter, _ *http.Request) {
		ready := configuration.Store != nil
		WriteJSON(w, http.StatusOK, healthResponse{Status: "ok", Service: "coreloop-api", Version: buildVersion, Ready: ready})
	}
	mux.HandleFunc("GET /api/app", health)
	mux.HandleFunc("GET /api/app/health", health)
	mux.HandleFunc("GET /api/app/ready", func(w http.ResponseWriter, r *http.Request) {
		if configuration.Store == nil {
			WriteJSON(w, http.StatusServiceUnavailable, healthResponse{Status: "degraded", Service: "coreloop-api", Version: buildVersion, Ready: false, Message: "runtime dependencies are not configured"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := configuration.Store.Ping(ctx); err != nil {
			WriteJSON(w, http.StatusServiceUnavailable, healthResponse{Status: "degraded", Service: "coreloop-api", Version: buildVersion, Ready: false, Message: "database is unavailable"})
			return
		}
		WriteJSON(w, http.StatusOK, healthResponse{Status: "ok", Service: "coreloop-api", Version: buildVersion, Ready: true})
	})
	if configuration.Store != nil && configuration.Auth != nil {
		mux.HandleFunc("GET /api/app/auth/start", configuration.startAuth)
		mux.HandleFunc("GET /api/app/auth/callback", configuration.authCallback)
		mux.Handle("GET /api/app/session", configuration.withSession(http.HandlerFunc(configuration.session)))
		mux.Handle("POST /api/app/auth/logout", configuration.withSession(configuration.withCSRF(http.HandlerFunc(configuration.logout))))
		mux.Handle("GET /api/app/topics", configuration.withSession(http.HandlerFunc(configuration.topics)))
		mux.Handle("GET /api/app/overview", configuration.withSession(http.HandlerFunc(configuration.overview)))
		mux.Handle("GET /api/app/profile", configuration.withSession(http.HandlerFunc(configuration.getProfile)))
		mux.Handle("PUT /api/app/profile", configuration.withSession(configuration.withCSRF(http.HandlerFunc(configuration.updateProfile))))
		mux.Handle("GET /api/app/preferences", configuration.withSession(http.HandlerFunc(configuration.getPreferences)))
		mux.Handle("PUT /api/app/preferences", configuration.withSession(configuration.withCSRF(http.HandlerFunc(configuration.updatePreferences))))
		mux.Handle("GET /api/app/progress", configuration.withSession(http.HandlerFunc(configuration.progress)))
		mux.Handle("POST /api/app/interactions", configuration.withSession(configuration.withCSRF(http.HandlerFunc(configuration.interaction))))
		mux.Handle("GET /api/app/export", configuration.withSession(http.HandlerFunc(configuration.export)))
		mux.Handle("DELETE /api/app/account", configuration.withSession(configuration.withCSRF(http.HandlerFunc(configuration.deleteAccount))))
		mux.Handle("POST /api/app/invites", configuration.withSession(configuration.withCSRF(configuration.ownerOnly(http.HandlerFunc(configuration.createInvite)))))
		mux.Handle("GET /api/app/operations", configuration.withSession(configuration.ownerOnly(http.HandlerFunc(configuration.operations))))
		mux.Handle("POST /api/app/operations/openai", configuration.withSession(configuration.withCSRF(configuration.ownerOnly(http.HandlerFunc(configuration.openAI)))))
	}
	return securityHeaders(routeCompatibility(mux))
}

func (configuration Config) startAuth(w http.ResponseWriter, r *http.Request) {
	loginURL, err := configuration.Auth.Start(r.Context(), r.URL.Query().Get("invite"), r.URL.Query().Get("return"))
	if err != nil {
		WriteProblem(w, apperror.Wrap(apperror.CodeInvalidRequest, err.Error(), http.StatusBadRequest, err))
		return
	}
	http.Redirect(w, r, loginURL, http.StatusFound)
}

func (configuration Config) authCallback(w http.ResponseWriter, r *http.Request) {
	result, err := configuration.Auth.Callback(r.Context(), r.URL.Query().Get("code"), r.URL.Query().Get("state"))
	if err != nil {
		slog.WarnContext(r.Context(), "Telegram login failed", "error", err)
		http.Redirect(w, r, "/access-required?error=login_failed", http.StatusFound)
		return
	}
	secure := configuration.Runtime.IsProduction()
	http.SetCookie(w, &http.Cookie{Name: auth.SessionCookieName, Value: result.SessionToken, Path: "/", Expires: result.SessionExpiry, MaxAge: int(time.Until(result.SessionExpiry).Seconds()), HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode})
	http.SetCookie(w, &http.Cookie{Name: auth.CSRFCookieName, Value: result.CSRFToken, Path: "/", Expires: result.SessionExpiry, MaxAge: int(time.Until(result.SessionExpiry).Seconds()), HttpOnly: false, Secure: secure, SameSite: http.SameSiteStrictMode})
	if result.Created && configuration.Telegram != nil {
		_, _ = configuration.Telegram.SendMessage(r.Context(), result.User.TelegramSubject, "Welcome to Coreloop. Your private profile is connected. Configure your topics and schedule in the web app; complete lessons will arrive here.", telegram.MessageOptions{})
	}
	http.Redirect(w, r, auth.SafeReturnPath(result.ReturnPath), http.StatusFound)
}

func (configuration Config) withSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(auth.SessionCookieName)
		if err != nil {
			WriteProblem(w, apperror.New(apperror.CodeUnauthorized, "sign in with Telegram to continue", http.StatusUnauthorized))
			return
		}
		session, user, err := configuration.Auth.Authenticate(r.Context(), cookie.Value)
		if err != nil {
			WriteProblem(w, apperror.New(apperror.CodeUnauthorized, "session is invalid or expired", http.StatusUnauthorized))
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), sessionKey, sessionContext{Session: session, User: user})))
	})
}

func (configuration Config) withCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" || origin != configuration.Runtime.AppOrigin {
			WriteProblem(w, apperror.New(apperror.CodeForbidden, "request origin is not allowed", http.StatusForbidden))
			return
		}
		value := sessionFrom(r)
		cookie, err := r.Cookie(auth.CSRFCookieName)
		if err != nil || !configuration.Auth.ValidateCSRF(value.Session, r.Header.Get("X-CSRF-Token")) || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(r.Header.Get("X-CSRF-Token"))) != 1 {
			WriteProblem(w, apperror.New(apperror.CodeForbidden, "CSRF token is invalid", http.StatusForbidden))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (configuration Config) ownerOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !configuration.Auth.IsOwner(sessionFrom(r).User) {
			WriteProblem(w, apperror.New(apperror.CodeForbidden, "owner access is required", http.StatusForbidden))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (configuration Config) session(w http.ResponseWriter, r *http.Request) {
	value := sessionFrom(r)
	WriteJSON(w, http.StatusOK, map[string]any{"user": value.User, "owner": configuration.Auth.IsOwner(value.User)})
}
func (configuration Config) logout(w http.ResponseWriter, r *http.Request) {
	value := sessionFrom(r)
	_ = configuration.Store.RevokeSession(r.Context(), value.Session.ID, time.Now())
	configuration.clearCookies(w)
	w.WriteHeader(http.StatusNoContent)
}
func (configuration Config) topics(w http.ResponseWriter, r *http.Request) {
	values, err := configuration.Store.Topics(r.Context())
	if err != nil {
		writeInternal(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"topics": values})
}
func (configuration Config) overview(w http.ResponseWriter, r *http.Request) {
	value, err := configuration.Store.Overview(r.Context(), sessionFrom(r).User.ID, time.Now())
	if err != nil {
		writeInternal(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, value)
}
func (configuration Config) getProfile(w http.ResponseWriter, r *http.Request) {
	profile, preferences, err := configuration.Store.Profile(r.Context(), sessionFrom(r).User.ID)
	if err != nil {
		writeInternal(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"profile": profile, "preferences": preferences})
}

func (configuration Config) updateProfile(w http.ResponseWriter, r *http.Request) {
	var profile store.LearningProfile
	if err := decodeJSON(r, &profile); err != nil {
		WriteProblem(w, apperror.New(apperror.CodeInvalidRequest, err.Error(), http.StatusBadRequest))
		return
	}
	if err := validateProfile(&profile); err != nil {
		WriteProblem(w, apperror.New(apperror.CodeInvalidRequest, err.Error(), http.StatusBadRequest))
		return
	}
	if err := configuration.Store.UpdateProfile(r.Context(), sessionFrom(r).User.ID, profile); err != nil {
		writeInternal(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, profile)
}
func (configuration Config) getPreferences(w http.ResponseWriter, r *http.Request) {
	preferences, err := configuration.Store.Preferences(r.Context(), sessionFrom(r).User.ID)
	if err != nil {
		writeInternal(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, preferences)
}
func (configuration Config) updatePreferences(w http.ResponseWriter, r *http.Request) {
	var preferences store.Preferences
	if err := decodeJSON(r, &preferences); err != nil {
		WriteProblem(w, apperror.New(apperror.CodeInvalidRequest, err.Error(), http.StatusBadRequest))
		return
	}
	if err := validatePreferences(preferences); err != nil {
		WriteProblem(w, apperror.New(apperror.CodeInvalidRequest, err.Error(), http.StatusBadRequest))
		return
	}
	if err := configuration.Store.UpdatePreferences(r.Context(), sessionFrom(r).User.ID, preferences); err != nil {
		WriteProblem(w, apperror.Wrap(apperror.CodeInvalidRequest, "preferences could not be saved", http.StatusBadRequest, err))
		return
	}
	WriteJSON(w, http.StatusOK, preferences)
}
func (configuration Config) progress(w http.ResponseWriter, r *http.Request) {
	userID := sessionFrom(r).User.ID
	items, err := configuration.Store.Assignments(r.Context(), userID, 100)
	if err != nil {
		writeInternal(w, err)
		return
	}
	dueRecall, err := configuration.Store.DueRecallCount(r.Context(), userID, time.Now())
	if err != nil {
		writeInternal(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"assignments": items, "due_recall": dueRecall})
}
func (configuration Config) interaction(w http.ResponseWriter, r *http.Request) {
	var input struct {
		AssignmentID string `json:"assignment_id"`
		Action       string `json:"action"`
	}
	if err := decodeJSON(r, &input); err != nil {
		WriteProblem(w, apperror.New(apperror.CodeInvalidRequest, err.Error(), http.StatusBadRequest))
		return
	}
	if input.Action != "read" && input.Action != "skip" {
		WriteProblem(w, apperror.New(apperror.CodeInvalidRequest, "action must be read or skip", http.StatusBadRequest))
		return
	}
	if err := configuration.Store.MarkAssignment(r.Context(), sessionFrom(r).User.ID, input.AssignmentID, input.Action, time.Now()); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (configuration Config) export(w http.ResponseWriter, r *http.Request) {
	value, err := configuration.Store.ExportUser(r.Context(), sessionFrom(r).User.ID)
	if err != nil {
		writeInternal(w, err)
		return
	}
	w.Header().Set("Content-Disposition", "attachment; filename=coreloop-export.json")
	WriteJSON(w, http.StatusOK, value)
}
func (configuration Config) deleteAccount(w http.ResponseWriter, r *http.Request) {
	userID := sessionFrom(r).User.ID
	if err := configuration.Store.DeleteUser(r.Context(), userID, time.Now()); err != nil {
		writeInternal(w, err)
		return
	}
	configuration.clearCookies(w)
	w.WriteHeader(http.StatusNoContent)
}

func (configuration Config) createInvite(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ExpiresHours int `json:"expires_hours"`
	}
	if err := decodeJSON(r, &input); err != nil {
		WriteProblem(w, apperror.New(apperror.CodeInvalidRequest, err.Error(), http.StatusBadRequest))
		return
	}
	if input.ExpiresHours <= 0 || input.ExpiresHours > 24*30 {
		input.ExpiresHours = 24 * 7
	}
	token, err := ids.Token(32)
	if err != nil {
		writeInternal(w, err)
		return
	}
	invite, err := configuration.Store.CreateInvite(r.Context(), securehash.Keyed(token, configuration.Runtime.SessionSecret), sessionFrom(r).User.ID, time.Now().Add(time.Duration(input.ExpiresHours)*time.Hour))
	if err != nil {
		writeInternal(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, map[string]any{"invite": invite, "url": configuration.Runtime.AppOrigin + "/invite/" + url.PathEscape(token)})
}
func (configuration Config) operations(w http.ResponseWriter, r *http.Request) {
	value, err := configuration.Store.Operations(r.Context())
	if err != nil {
		writeInternal(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, value)
}
func (configuration Config) openAI(w http.ResponseWriter, r *http.Request) {
	if configuration.Jobs == nil {
		WriteProblem(w, apperror.New(apperror.CodeNotReady, "job service is not configured", http.StatusServiceUnavailable))
		return
	}
	var input struct {
		JobID string `json:"job_id"`
	}
	if err := decodeJSON(r, &input); err != nil || input.JobID == "" {
		WriteProblem(w, apperror.New(apperror.CodeInvalidRequest, "job_id is required", http.StatusBadRequest))
		return
	}
	if err := configuration.Jobs.RunBlockedWithOpenAI(r.Context(), input.JobID); err != nil {
		WriteProblem(w, apperror.Wrap(apperror.CodeConflict, err.Error(), http.StatusConflict, err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (configuration Config) clearCookies(w http.ResponseWriter) {
	secure := configuration.Runtime.IsProduction()
	for _, name := range []string{auth.SessionCookieName, auth.CSRFCookieName} {
		http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(1, 0), HttpOnly: name == auth.SessionCookieName, Secure: secure, SameSite: http.SameSiteLaxMode})
	}
}
func sessionFrom(r *http.Request) sessionContext {
	return r.Context().Value(sessionKey).(sessionContext)
}

func validateProfile(profile *store.LearningProfile) error {
	switch profile.CurrentLevel {
	case "beginner", "intermediate", "advanced":
	default:
		return errors.New("current_level is invalid")
	}
	lists := [][]string{profile.Goals, profile.TargetRoles, profile.CurrentTechnologies, profile.TargetTechnologies}
	for _, list := range lists {
		if len(list) > 20 {
			return errors.New("profile lists can contain at most 20 items")
		}
		for _, value := range list {
			if len([]rune(strings.TrimSpace(value))) > 120 {
				return errors.New("profile items can contain at most 120 characters")
			}
		}
	}
	return nil
}
func validatePreferences(value store.Preferences) error {
	if value.LessonMinutes != 15 && value.LessonMinutes != 30 {
		return errors.New("lesson_minutes must be 15 or 30")
	}
	if value.LessonsPerDay < 1 || value.LessonsPerDay > 6 {
		return errors.New("lessons_per_day must be between 1 and 6")
	}
	if len(value.DeliveryTimes) != value.LessonsPerDay {
		return errors.New("delivery_times must match lessons_per_day")
	}
	if value.TimeZone != "Asia/Kolkata" {
		return errors.New("time_zone must be Asia/Kolkata")
	}
	if value.ExplanationDepth != "foundation" && value.ExplanationDepth != "standard" && value.ExplanationDepth != "detailed" {
		return errors.New("explanation_depth is invalid")
	}
	if value.RecallMode != "off" && value.RecallMode != "light" && value.RecallMode != "standard" {
		return errors.New("recall_mode is invalid")
	}
	if value.BundleMode != "complete" && value.BundleMode != "continue_after_intro" {
		return errors.New("bundle_mode is invalid")
	}
	if len(value.TopicIDs) == 0 {
		return errors.New("choose at least one topic")
	}
	seen := map[string]bool{}
	for _, timeValue := range value.DeliveryTimes {
		if seen[timeValue] {
			return errors.New("delivery times must be unique")
		}
		seen[timeValue] = true
		parsed, err := time.Parse("15:04", timeValue)
		if err != nil || parsed.Format("15:04") != timeValue {
			return errors.New("delivery times must use HH:MM")
		}
	}
	return nil
}

func decodeJSON(r *http.Request, destination any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("request must contain one JSON object")
	}
	return nil
}
func writeStoreError(w http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		WriteProblem(w, apperror.New(apperror.CodeNotFound, "resource was not found", http.StatusNotFound))
		return
	}
	writeInternal(w, err)
}
func writeInternal(w http.ResponseWriter, err error) {
	slog.Error("request failed", "error", err)
	WriteProblem(w, apperror.New(apperror.CodeInternal, "the request could not be completed", http.StatusInternalServerError))
}

func routeCompatibility(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if path := strings.Trim(r.URL.Query().Get("__path"), "/"); path != "" {
			copy := r.Clone(r.Context())
			copy.URL.Path = "/api/app/" + path
			r = copy
		}
		next.ServeHTTP(w, r)
	})
}
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}

func NewNotReadyHandler(component string) http.Handler {
	message := component + " is not configured"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			WriteProblem(w, apperror.New(apperror.CodeMethodNotAllowed, "only POST is accepted", http.StatusMethodNotAllowed))
			return
		}
		WriteProblem(w, apperror.New(apperror.CodeNotReady, message, http.StatusNotImplemented))
	})
}

var _ = strconv.Itoa
