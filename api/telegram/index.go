package handler

import (
	"net/http"
	"sync"

	"coreloop/backend/app"
)

var (
	telegramHandlerOnce sync.Once
	telegramHandler     http.Handler
)

func Handler(w http.ResponseWriter, r *http.Request) {
	telegramHandlerOnce.Do(func() {
		telegramHandler = app.NewTelegramHandler()
	})
	telegramHandler.ServeHTTP(w, r)
}
