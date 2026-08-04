package handler

import (
	"net/http"
	"os"
	"sync"

	"coreloop/backend/app"
)

var (
	appHandlerOnce sync.Once
	appHandler     http.Handler
)

func Handler(w http.ResponseWriter, r *http.Request) {
	appHandlerOnce.Do(func() {
		appHandler = app.NewHTTPHandler(os.Getenv("VERCEL_GIT_COMMIT_SHA"))
	})
	appHandler.ServeHTTP(w, r)
}
