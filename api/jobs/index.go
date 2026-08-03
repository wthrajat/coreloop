package handler

import (
	"net/http"

	"coreloop/backend/app"
)

var jobsHandler = app.NewJobsHandler()

func Handler(w http.ResponseWriter, r *http.Request) {
	jobsHandler.ServeHTTP(w, r)
}
