package app

import (
	"log/slog"
	"net/http"
	"sync"

	"coreloop/backend/internal/alerts"
	"coreloop/backend/internal/auth"
	"coreloop/backend/internal/config"
	"coreloop/backend/internal/database/tursohttp"
	"coreloop/backend/internal/httpapi"
	"coreloop/backend/internal/jobs"
	"coreloop/backend/internal/providers"
	"coreloop/backend/internal/qstash"
	"coreloop/backend/internal/store"
	"coreloop/backend/internal/telegram"
)

type runtime struct {
	configuration config.Config
	store         *store.Store
	auth          *auth.Service
	telegram      *telegram.Client
	jobs          *jobs.Service
	receiver      *qstash.Receiver
	err           error
}

var runtimeOnce sync.Once
var shared runtime

func dependencies() runtime {
	runtimeOnce.Do(func() {
		configuration := config.FromEnv()
		shared.configuration = configuration
		if err := configuration.ValidateRuntime(); err != nil {
			shared.err = err
			slog.Warn("runtime is not fully configured", "error", err)
			return
		}
		database, err := tursohttp.Open(configuration.TursoURL, configuration.TursoToken, nil)
		if err != nil {
			shared.err = err
			return
		}
		dataStore := store.New(database)
		telegramClient := telegram.New(configuration.TelegramBotToken, nil)
		providerRouter := providers.NewRouter(providers.NewGroq(configuration.GroqAPIKey, configuration.GroqModel, nil), providers.NewGemini(configuration.GeminiAPIKey, configuration.GeminiModel, nil), providers.NewOpenAI(configuration.OpenAIAPIKey, configuration.OpenAIModel, nil))
		publisher := qstash.NewPublisher(configuration.QStashToken, nil)
		receiver := qstash.NewReceiver(configuration.QStashCurrentSigningKey, configuration.QStashNextSigningKey)
		jobService := jobs.New(dataStore, providerRouter, telegramClient, publisher, alerts.FromEnv(configuration.AdminAlertEmail), configuration.AppOrigin)
		shared.store = dataStore
		shared.telegram = telegramClient
		shared.auth = auth.NewService(dataStore, configuration, nil)
		shared.jobs = jobService
		shared.receiver = receiver
	})
	return shared
}

func NewHTTPHandler(buildVersion string) http.Handler {
	value := dependencies()
	if buildVersion == "" {
		buildVersion = value.configuration.BuildVersion
	}
	return httpapi.NewRouter(httpapi.Config{BuildVersion: buildVersion, Runtime: value.configuration, Store: value.store, Auth: value.auth, Jobs: value.jobs, Telegram: value.telegram})
}

func NewJobsHandler() http.Handler {
	value := dependencies()
	if value.jobs == nil {
		return httpapi.NewNotReadyHandler("QStash job worker")
	}
	return httpapi.NewJobsRouter(httpapi.JobsConfig{AppOrigin: value.configuration.AppOrigin, Receiver: value.receiver, Jobs: value.jobs})
}
func NewTelegramHandler() http.Handler {
	value := dependencies()
	if value.telegram == nil {
		return httpapi.NewNotReadyHandler("Telegram webhook")
	}
	return httpapi.NewTelegramRouter(httpapi.TelegramConfig{WebhookSecret: value.configuration.TelegramWebhookSecret, Telegram: value.telegram, Store: value.store})
}
func NewNotReadyHandler(component string) http.Handler { return httpapi.NewNotReadyHandler(component) }
