package httpapi

import (
	"encoding/json"
	"net/http"

	"coreloop/backend/internal/apperror"
)

type problemResponse struct {
	Error problemDetails `json:"error"`
}

type problemDetails struct {
	Code    apperror.Code `json:"code"`
	Message string        `json:"message"`
}

func WriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(value); err != nil {
		return
	}
}

func WriteProblem(w http.ResponseWriter, err *apperror.Error) {
	WriteJSON(w, err.HTTPStatus, problemResponse{
		Error: problemDetails{
			Code:    err.Code,
			Message: err.Message,
		},
	})
}
