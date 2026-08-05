package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"coreloop/backend/internal/jobs"
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
