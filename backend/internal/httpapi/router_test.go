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
	userID    string
	requestID string
}

func (fake *fakeApplicationJobs) RunBlockedWithOpenAI(context.Context, string) error {
	return nil
}

func (fake *fakeApplicationJobs) TriggerLessonNow(_ context.Context, userID, requestID string) (jobs.ManualLesson, error) {
	fake.userID = userID
	fake.requestID = requestID
	return jobs.ManualLesson{JobID: "job_manual", State: "queued", Message: "Queued for immediate generation."}, nil
}

func (fake *fakeApplicationJobs) ManualLessonStatus(context.Context, string, string) (jobs.ManualLesson, error) {
	return jobs.ManualLesson{}, nil
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
	if jobService.userID != "usr_owner" || jobService.requestID != "request-123" {
		t.Fatalf("unexpected trigger input: %#v", jobService)
	}
}
