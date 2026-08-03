package httpapi

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"coreloop/backend/internal/apperror"
	"coreloop/backend/internal/jobs"
	"coreloop/backend/internal/qstash"
	"coreloop/backend/internal/store"
	"coreloop/backend/internal/telegram"
)

type JobsConfig struct {
	AppOrigin string
	Receiver  *qstash.Receiver
	Jobs      *jobs.Service
}

func NewJobsRouter(configuration JobsConfig) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/jobs/tick", func(w http.ResponseWriter, r *http.Request) {
		body, ok := verifyQStash(w, r, configuration)
		if !ok {
			return
		}
		_ = body
		if err := configuration.Jobs.Tick(r.Context()); err != nil {
			writeInternal(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]string{"status": "accepted"})
	})
	mux.HandleFunc("POST /api/jobs/run", func(w http.ResponseWriter, r *http.Request) {
		body, ok := verifyQStash(w, r, configuration)
		if !ok {
			return
		}
		var input struct {
			JobID string `json:"job_id"`
		}
		if err := json.Unmarshal(body, &input); err != nil || input.JobID == "" {
			WriteProblem(w, apperror.New(apperror.CodeInvalidRequest, "job_id is required", http.StatusBadRequest))
			return
		}
		if err := configuration.Jobs.Run(r.Context(), input.JobID, "qstash:"+r.Header.Get("Upstash-Message-Id")); err != nil {
			slog.ErrorContext(r.Context(), "job failed", "job_id", input.JobID, "error", err)
			WriteProblem(w, apperror.New(apperror.CodeInternal, "job execution failed", http.StatusInternalServerError))
			return
		}
		WriteJSON(w, http.StatusOK, map[string]string{"status": "completed"})
	})
	return securityHeaders(routePrefixCompatibility(mux, "/api/jobs"))
}

func verifyQStash(w http.ResponseWriter, r *http.Request, configuration JobsConfig) ([]byte, bool) {
	if configuration.Receiver == nil || configuration.Jobs == nil {
		WriteProblem(w, apperror.New(apperror.CodeNotReady, "job service is not configured", http.StatusServiceUnavailable))
		return nil, false
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		WriteProblem(w, apperror.New(apperror.CodeInvalidRequest, "request body is too large", http.StatusBadRequest))
		return nil, false
	}
	expected := strings.TrimRight(configuration.AppOrigin, "/") + r.URL.Path
	if err := configuration.Receiver.Verify(r.Header.Get("Upstash-Signature"), expected, body); err != nil {
		WriteProblem(w, apperror.New(apperror.CodeForbidden, "QStash signature is invalid", http.StatusForbidden))
		return nil, false
	}
	return body, true
}

type TelegramConfig struct {
	WebhookSecret string
	Telegram      *telegram.Client
	Store         *store.Store
}

func NewTelegramRouter(configuration TelegramConfig) http.Handler {
	return securityHeaders(routePrefixCompatibility(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			WriteProblem(w, apperror.New(apperror.CodeMethodNotAllowed, "only POST is accepted", http.StatusMethodNotAllowed))
			return
		}
		if configuration.Telegram == nil || configuration.Store == nil {
			WriteProblem(w, apperror.New(apperror.CodeNotReady, "Telegram is not configured", http.StatusServiceUnavailable))
			return
		}
		provided := r.Header.Get("X-Telegram-Bot-Api-Secret-Token")
		if len(provided) != len(configuration.WebhookSecret) || subtle.ConstantTimeCompare([]byte(provided), []byte(configuration.WebhookSecret)) != 1 {
			WriteProblem(w, apperror.New(apperror.CodeForbidden, "Telegram webhook secret is invalid", http.StatusForbidden))
			return
		}
		var update telegram.Update
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		if err := decoder.Decode(&update); err != nil {
			WriteProblem(w, apperror.New(apperror.CodeInvalidRequest, "invalid Telegram update", http.StatusBadRequest))
			return
		}
		if update.CallbackQuery == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		callback := update.CallbackQuery
		user, err := configuration.Store.UserByTelegramSubject(r.Context(), strconv.FormatInt(callback.From.ID, 10))
		if err != nil {
			_ = configuration.Telegram.AnswerCallback(r.Context(), callback.ID, "This profile is not connected.")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		message := "Saved"
		switch {
		case strings.HasPrefix(callback.Data, "read:"):
			err = configuration.Store.MarkAssignment(r.Context(), user.ID, strings.TrimPrefix(callback.Data, "read:"), "read", now())
			message = "Marked as read"
		case strings.HasPrefix(callback.Data, "skip:"):
			err = configuration.Store.MarkAssignment(r.Context(), user.ID, strings.TrimPrefix(callback.Data, "skip:"), "skip", now())
			message = "Skipped"
		case strings.HasPrefix(callback.Data, "radar_skip:"):
			err = configuration.Store.CompleteRadar(r.Context(), user.ID, strings.TrimPrefix(callback.Data, "radar_skip:"), "skipped", now())
			message = "Signal skipped"
		default:
			err = fmt.Errorf("unknown callback action")
		}
		if err != nil {
			message = "Could not save that action"
		}
		_ = configuration.Telegram.AnswerCallback(r.Context(), callback.ID, message)
		w.WriteHeader(http.StatusNoContent)
	}), "/api/telegram"))
}

func routePrefixCompatibility(next http.Handler, prefix string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if path := strings.Trim(r.URL.Query().Get("__path"), "/"); path != "" {
			copy := r.Clone(r.Context())
			copy.URL.Path = prefix + "/" + path
			r = copy
		}
		next.ServeHTTP(w, r)
	})
}

func now() time.Time { return time.Now() }
