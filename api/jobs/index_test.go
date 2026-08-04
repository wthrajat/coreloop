package handler

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestHandlerConstructionIsDeferredUntilTheFirstRequest(t *testing.T) {
	previousConstructor := newJobsHandler
	t.Cleanup(func() {
		jobsHandlerOnce = sync.Once{}
		jobsHandler = nil
		newJobsHandler = previousConstructor
	})

	jobsHandlerOnce = sync.Once{}
	jobsHandler = nil
	constructionCount := 0
	newJobsHandler = func() http.Handler {
		constructionCount++
		return http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
			response.WriteHeader(http.StatusNoContent)
		})
	}

	if constructionCount != 0 {
		t.Fatalf("handler constructed before a request: %d", constructionCount)
	}
	for range 2 {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/jobs/tick", nil)
		Handler(response, request)
		if response.Code != http.StatusNoContent {
			t.Fatalf("response status = %d, want %d", response.Code, http.StatusNoContent)
		}
	}
	if constructionCount != 1 {
		t.Fatalf("handler construction count = %d, want 1", constructionCount)
	}
}
