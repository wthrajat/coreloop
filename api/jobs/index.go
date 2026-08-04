package handler

import (
	"net/http"
	"sync"

	"coreloop/backend/app"
)

var (
	jobsHandlerOnce sync.Once
	jobsHandler     http.Handler
	newJobsHandler  = app.NewJobsHandler
)

func Handler(w http.ResponseWriter, r *http.Request) {
	jobsHandlerOnce.Do(func() {
		jobsHandler = newJobsHandler()
	})
	jobsHandler.ServeHTTP(w, r)
}
