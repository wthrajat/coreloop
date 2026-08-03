package handler

import (
	"net/http"
	"os"

	"coreloop/backend/app"
)

var appHandler = app.NewHTTPHandler(os.Getenv("VERCEL_GIT_COMMIT_SHA"))

func Handler(w http.ResponseWriter, r *http.Request) {
	appHandler.ServeHTTP(w, r)
}
