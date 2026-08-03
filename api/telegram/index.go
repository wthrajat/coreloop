package handler

import (
	"net/http"

	"coreloop/backend/app"
)

var telegramHandler = app.NewTelegramHandler()

func Handler(w http.ResponseWriter, r *http.Request) {
	telegramHandler.ServeHTTP(w, r)
}
