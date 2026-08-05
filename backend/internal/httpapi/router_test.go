package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"coreloop/backend/internal/auth"
	"coreloop/backend/internal/config"
	"coreloop/backend/internal/jobs"
	"coreloop/backend/internal/securehash"
	"coreloop/backend/internal/store"
)

type fakeApplicationJobs struct {
	lessonUserID    string
	lessonRequestID string
	radarUserID     string
	radarRequestID  string
	radarError      error
}

func (fake *fakeApplicationJobs) RunBlockedWithOpenAI(context.Context, string) error {
	return nil
}

func (fake *fakeApplicationJobs) TriggerLessonNow(_ context.Context, userID, requestID string) (jobs.ManualLesson, error) {
	fake.lessonUserID = userID
	fake.lessonRequestID = requestID
	return jobs.ManualLesson{JobID: "job_manual", State: "queued", Message: "Queued for immediate generation."}, nil
}

func (fake *fakeApplicationJobs) ManualLessonStatus(context.Context, string, string) (jobs.ManualLesson, error) {
	return jobs.ManualLesson{}, nil
}

func (fake *fakeApplicationJobs) TriggerRadarNow(_ context.Context, userID, requestID string) (jobs.ManualRadar, error) {
	fake.radarUserID = userID
	fake.radarRequestID = requestID
	if fake.radarError != nil {
		return jobs.ManualRadar{}, fake.radarError
	}
	return jobs.ManualRadar{
		BatchID: "batch_radar", State: "queued", ProfileTarget: 8,
		RequestedCount: 8, SelectedCount: 8, Message: "Queued for Telegram.",
	}, nil
}

func (fake *fakeApplicationJobs) ManualRadarStatus(context.Context, string, string) (jobs.ManualRadar, error) {
	return jobs.ManualRadar{}, nil
}

func TestHealthEndpoint(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/app/health", nil)
	response := httptest.NewRecorder()

	handler := NewRouter(Config{BuildVersion: "test-build"})
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	var payload healthResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode health response: %v", err)
	}

	if payload.Status != "ok" || payload.Version != "test-build" {
		t.Fatalf("unexpected health payload: %#v", payload)
	}
	for header, expected := range map[string]string{
		"Cache-Control":                     "no-store",
		"Cross-Origin-Opener-Policy":        "same-origin",
		"Cross-Origin-Resource-Policy":      "same-origin",
		"X-Content-Type-Options":            "nosniff",
		"X-Frame-Options":                   "DENY",
		"X-Permitted-Cross-Domain-Policies": "none",
		"X-Robots-Tag":                      "noindex, nofollow, noarchive",
	} {
		if actual := response.Header().Get(header); actual != expected {
			t.Errorf("%s = %q, want %q", header, actual, expected)
		}
	}
}

func TestRequestClientAddressUsesTrustedVercelAddress(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-Vercel-Forwarded-For", "198.51.100.9")
	if address := requestClientAddress(request, true); address != "198.51.100.9" {
		t.Fatalf("client address = %q", address)
	}
}

func TestRequestClientAddressIgnoresForwardingHeadersOutsideVercel(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "203.0.113.10:4321"
	request.Header.Set("X-Forwarded-For", "198.51.100.9")
	if address := requestClientAddress(request, false); address != "203.0.113.10" {
		t.Fatalf("client address = %q", address)
	}
}

func TestReadinessProbeCoalescesChecksWithinTTL(t *testing.T) {
	var calls int
	probe := newReadinessProbe(func(context.Context) error {
		calls++
		return nil
	}, 10*time.Second)
	checkedAt := time.Date(2026, time.August, 5, 10, 0, 0, 0, time.UTC)

	var group sync.WaitGroup
	for range 10 {
		group.Add(1)
		go func() {
			defer group.Done()
			if err := probe.Check(context.Background(), checkedAt); err != nil {
				t.Errorf("readiness check failed: %v", err)
			}
		}()
	}
	group.Wait()
	if calls != 1 {
		t.Fatalf("readiness backend calls = %d, want 1", calls)
	}
}

func TestLoginBindingCookieIsHostOnlyAndHardened(t *testing.T) {
	configuration := Config{Runtime: config.Config{AppOrigin: "https://coreloop.example"}}
	response := httptest.NewRecorder()
	configuration.setLoginBindingCookie(response, "binding-token", time.Now().Add(10*time.Minute))

	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %d, want 1", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != auth.SecureLoginBindingName || cookie.Domain != "" ||
		cookie.Path != "/" || !cookie.Secure || !cookie.HttpOnly ||
		cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("login-binding cookie is not hardened: %#v", cookie)
	}
}

func TestSessionCookiesUseTheExpectedSecurityPolicy(t *testing.T) {
	configuration := Config{Runtime: config.Config{AppOrigin: "https://coreloop.example"}}
	response := httptest.NewRecorder()
	configuration.setSessionCookies(response, auth.LoginResult{
		SessionToken:  "session-token",
		CSRFToken:     "csrf-token",
		SessionExpiry: time.Now().Add(time.Hour),
	})

	cookies := response.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("cookies = %d, want 2", len(cookies))
	}
	byName := map[string]*http.Cookie{}
	for _, cookie := range cookies {
		byName[cookie.Name] = cookie
	}
	sessionCookie := byName[auth.SessionCookieName]
	csrfCookie := byName[auth.CSRFCookieName]
	if sessionCookie == nil || !sessionCookie.Secure || !sessionCookie.HttpOnly ||
		sessionCookie.Domain != "" || sessionCookie.Path != "/" ||
		sessionCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("session cookie is not hardened: %#v", sessionCookie)
	}
	if csrfCookie == nil || !csrfCookie.Secure || csrfCookie.HttpOnly ||
		csrfCookie.Domain != "" || csrfCookie.Path != "/" ||
		csrfCookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("CSRF cookie policy is invalid: %#v", csrfCookie)
	}
}

func TestCSRFRequiresExactOriginCookieHeaderAndSessionHash(t *testing.T) {
	const (
		origin = "https://coreloop.example"
		token  = "csrf-token"
		secret = "test-session-secret-with-32-characters"
	)
	authentication := auth.NewService(nil, config.Config{SessionSecret: secret}, nil)
	configuration := Config{
		Runtime: config.Config{AppOrigin: origin},
		Auth:    authentication,
	}
	handler := configuration.withCSRF(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	session := store.Session{CSRFHash: securehash.Keyed(token, secret)}

	for name, setup := range map[string]func(*http.Request){
		"wrong origin": func(request *http.Request) {
			request.Header.Set("Origin", "https://evil.example")
			request.Header.Set("X-CSRF-Token", token)
			request.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: token})
		},
		"missing cookie": func(request *http.Request) {
			request.Header.Set("Origin", origin)
			request.Header.Set("X-CSRF-Token", token)
		},
		"mismatched header": func(request *http.Request) {
			request.Header.Set("Origin", origin)
			request.Header.Set("X-CSRF-Token", "other-token")
			request.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: token})
		},
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/", nil)
			setup(request)
			request = request.WithContext(context.WithValue(
				request.Context(),
				sessionKey,
				sessionContext{Session: session},
			))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
			}
		})
	}

	request := httptest.NewRequest(http.MethodPost, "/", nil)
	request.Header.Set("Origin", origin)
	request.Header.Set("X-CSRF-Token", token)
	request.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: token})
	request = request.WithContext(context.WithValue(
		request.Context(),
		sessionKey,
		sessionContext{Session: session},
	))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("valid CSRF status = %d, want %d", response.Code, http.StatusNoContent)
	}
}

func TestOwnerOnlyRejectsARegularUser(t *testing.T) {
	authentication := auth.NewService(nil, config.Config{
		OwnerTelegramSubject: "12345",
	}, nil)
	configuration := Config{Auth: authentication}
	handler := configuration.ownerOnly(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request = request.WithContext(context.WithValue(
		request.Context(),
		sessionKey,
		sessionContext{User: store.User{TelegramSubject: "67890"}},
	))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestNotReadyHandlerRejectsGet(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/jobs", nil)
	response := httptest.NewRecorder()

	NewNotReadyHandler("job worker").ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusMethodNotAllowed,
			response.Code,
		)
	}
}

func TestTriggerLessonNowUsesAuthenticatedUser(t *testing.T) {
	jobService := &fakeApplicationJobs{}
	configuration := Config{Jobs: jobService}
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/app/operations/lessons",
		bytes.NewBufferString(`{"request_id":"request-123"}`),
	)
	request = request.WithContext(context.WithValue(request.Context(), sessionKey, sessionContext{
		User: store.User{ID: "usr_owner"},
	}))
	response := httptest.NewRecorder()

	configuration.triggerLessonNow(response, request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d: %s", http.StatusAccepted, response.Code, response.Body.String())
	}
	if jobService.lessonUserID != "usr_owner" || jobService.lessonRequestID != "request-123" {
		t.Fatalf("unexpected trigger input: %#v", jobService)
	}
}

func TestTriggerRadarNowUsesAuthenticatedUser(t *testing.T) {
	jobService := &fakeApplicationJobs{}
	configuration := Config{Jobs: jobService}
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/app/operations/radar",
		bytes.NewBufferString(`{"request_id":"radar-request-123"}`),
	)
	request = request.WithContext(context.WithValue(request.Context(), sessionKey, sessionContext{
		User: store.User{ID: "usr_owner"},
	}))
	response := httptest.NewRecorder()

	configuration.triggerRadarNow(response, request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d: %s", http.StatusAccepted, response.Code, response.Body.String())
	}
	if jobService.radarUserID != "usr_owner" || jobService.radarRequestID != "radar-request-123" {
		t.Fatalf("unexpected trigger input: %#v", jobService)
	}
}

func TestTriggerRadarNowReportsUnavailableCandidate(t *testing.T) {
	jobService := &fakeApplicationJobs{radarError: jobs.ErrManualRadarUnavailable}
	configuration := Config{Jobs: jobService}
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/app/operations/radar",
		bytes.NewBufferString(`{"request_id":"radar-request-456"}`),
	)
	request = request.WithContext(context.WithValue(request.Context(), sessionKey, sessionContext{
		User: store.User{ID: "usr_owner"},
	}))
	response := httptest.NewRecorder()

	configuration.triggerRadarNow(response, request)

	if response.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d: %s", http.StatusConflict, response.Code, response.Body.String())
	}
	var payload struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode conflict response: %v", err)
	}
	if payload.Error.Code != "conflict" || payload.Error.Message == "" {
		t.Fatalf("unexpected conflict payload: %#v", payload)
	}
}

func TestValidatePreferencesRejectsInvalidRadarItemsPerDay(t *testing.T) {
	preferences := store.Preferences{
		LessonMinutes:    15,
		ExplanationDepth: "standard",
		LessonsPerDay:    1,
		RecallMode:       "light",
		BundleMode:       "complete",
		TimeZone:         "Asia/Kolkata",
		DeliveryTimes:    []string{"20:30"},
		TopicIDs:         []string{"topic_backend"},
	}

	for _, itemsPerDay := range []int{-1, 51} {
		preferences.RadarItemsPerDay = itemsPerDay
		if err := validatePreferences(preferences); err == nil {
			t.Fatalf("expected radar_items_per_day=%d to be rejected", itemsPerDay)
		}
	}

	for _, itemsPerDay := range []int{0, 8, 50} {
		preferences.RadarItemsPerDay = itemsPerDay
		if err := validatePreferences(preferences); err != nil {
			t.Fatalf("expected radar_items_per_day=%d to be accepted: %v", itemsPerDay, err)
		}
	}
}
