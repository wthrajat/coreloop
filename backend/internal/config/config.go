package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const defaultTimeZone = "Asia/Kolkata"

type Config struct {
	Environment       string
	VercelEnvironment string
	AppOrigin         string
	BuildVersion      string

	TursoURL   string
	TursoToken string

	TelegramClientID      string
	TelegramClientSecret  string
	TelegramBotToken      string
	TelegramWebhookSecret string
	OwnerTelegramSubject  string

	QStashCurrentSigningKey string
	QStashNextSigningKey    string
	QStashToken             string

	GroqAPIKey   string
	GroqModel    string
	GeminiAPIKey string
	GeminiModel  string
	OpenAIAPIKey string
	OpenAIModel  string

	AdminAlertEmail string
	SessionSecret   string
	TimeZone        string
}

func FromEnv() Config {
	environment := strings.TrimSpace(os.Getenv("APP_ENV"))
	vercelEnvironment := strings.TrimSpace(os.Getenv("VERCEL_ENV"))
	if environment == "" {
		if vercelEnvironment == "production" {
			environment = "production"
		} else {
			environment = "development"
		}
	}
	return Config{
		Environment:             environment,
		VercelEnvironment:       vercelEnvironment,
		AppOrigin:               strings.TrimRight(envOr("APP_ORIGIN", "http://localhost:3000"), "/"),
		BuildVersion:            envOr("VERCEL_GIT_COMMIT_SHA", "development"),
		TursoURL:                os.Getenv("TURSO_DATABASE_URL"),
		TursoToken:              os.Getenv("TURSO_AUTH_TOKEN"),
		TelegramClientID:        os.Getenv("TELEGRAM_CLIENT_ID"),
		TelegramClientSecret:    os.Getenv("TELEGRAM_CLIENT_SECRET"),
		TelegramBotToken:        os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramWebhookSecret:   os.Getenv("TELEGRAM_WEBHOOK_SECRET"),
		OwnerTelegramSubject:    os.Getenv("OWNER_TELEGRAM_SUBJECT"),
		QStashCurrentSigningKey: os.Getenv("QSTASH_CURRENT_SIGNING_KEY"),
		QStashNextSigningKey:    os.Getenv("QSTASH_NEXT_SIGNING_KEY"),
		QStashToken:             os.Getenv("QSTASH_TOKEN"),
		GroqAPIKey:              os.Getenv("GROQ_API_KEY"),
		GroqModel:               envOr("GROQ_MODEL", "openai/gpt-oss-20b"),
		GeminiAPIKey:            os.Getenv("GEMINI_API_KEY"),
		GeminiModel:             envOr("GEMINI_MODEL", "gemini-3.6-flash"),
		OpenAIAPIKey:            os.Getenv("OPENAI_API_KEY"),
		OpenAIModel:             envOr("OPENAI_MODEL", "gpt-5.6-terra"),
		AdminAlertEmail:         os.Getenv("ADMIN_ALERT_EMAIL"),
		SessionSecret:           os.Getenv("SESSION_SECRET"),
		TimeZone:                envOr("APP_TIME_ZONE", defaultTimeZone),
	}
}

func (config Config) ValidateProduction() error {
	var missing []string
	if config.Environment != "production" {
		missing = append(missing, "APP_ENV (must be production)")
	}
	if config.VercelEnvironment != "" && config.VercelEnvironment != "production" {
		missing = append(missing, "VERCEL_ENV (must agree with APP_ENV)")
	}
	for name, value := range map[string]string{
		"APP_ORIGIN":                 config.AppOrigin,
		"TURSO_DATABASE_URL":         config.TursoURL,
		"TURSO_AUTH_TOKEN":           config.TursoToken,
		"TELEGRAM_CLIENT_ID":         config.TelegramClientID,
		"TELEGRAM_CLIENT_SECRET":     config.TelegramClientSecret,
		"TELEGRAM_BOT_TOKEN":         config.TelegramBotToken,
		"TELEGRAM_WEBHOOK_SECRET":    config.TelegramWebhookSecret,
		"OWNER_TELEGRAM_SUBJECT":     config.OwnerTelegramSubject,
		"QSTASH_CURRENT_SIGNING_KEY": config.QStashCurrentSigningKey,
		"QSTASH_NEXT_SIGNING_KEY":    config.QStashNextSigningKey,
		"QSTASH_TOKEN":               config.QStashToken,
		"SESSION_SECRET":             config.SessionSecret,
	} {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, name)
		}
	}
	if config.GroqAPIKey == "" && config.GeminiAPIKey == "" {
		missing = append(missing, "GROQ_API_KEY or GEMINI_API_KEY")
	}
	if config.SessionSecret != "" && len(config.SessionSecret) < 32 {
		missing = append(missing, "SESSION_SECRET (at least 32 characters)")
	}
	if config.TelegramWebhookSecret != "" && len(config.TelegramWebhookSecret) < 32 {
		missing = append(missing, "TELEGRAM_WEBHOOK_SECRET (at least 32 characters)")
	}
	if config.QStashCurrentSigningKey != "" && len(config.QStashCurrentSigningKey) < 32 {
		missing = append(missing, "QSTASH_CURRENT_SIGNING_KEY (at least 32 characters)")
	}
	if config.QStashNextSigningKey != "" && len(config.QStashNextSigningKey) < 32 {
		missing = append(missing, "QSTASH_NEXT_SIGNING_KEY (at least 32 characters)")
	}
	ownerSubject, ownerSubjectError := strconv.ParseInt(config.OwnerTelegramSubject, 10, 64)
	if ownerSubjectError != nil || ownerSubject <= 0 {
		missing = append(missing, "OWNER_TELEGRAM_SUBJECT (numeric)")
	}
	parsedOrigin, err := url.Parse(config.AppOrigin)
	if err != nil || parsedOrigin.Host == "" || parsedOrigin.Scheme != "https" ||
		parsedOrigin.User != nil || parsedOrigin.Path != "" || parsedOrigin.RawQuery != "" ||
		parsedOrigin.Fragment != "" {
		missing = append(missing, "APP_ORIGIN (https origin without path, query, or fragment)")
	}
	databaseURL := strings.Replace(config.TursoURL, "libsql://", "https://", 1)
	parsedDatabase, databaseError := url.Parse(databaseURL)
	if databaseError != nil || parsedDatabase.Host == "" || parsedDatabase.Scheme != "https" {
		missing = append(missing, "TURSO_DATABASE_URL (libsql or https URL)")
	}
	if config.TimeZone != defaultTimeZone {
		if _, err := time.LoadLocation(config.TimeZone); err != nil {
			missing = append(missing, "APP_TIME_ZONE (valid IANA zone)")
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("invalid production configuration: %s", strings.Join(missing, ", "))
	}
	return nil
}

func (config Config) IsProduction() bool {
	return config.Environment == "production" || config.VercelEnvironment == "production"
}

func (config Config) SecureCookies() bool {
	origin, err := url.Parse(config.AppOrigin)
	return err == nil && origin.Scheme == "https" && origin.Host != ""
}

func (config Config) ValidateRuntime() error {
	switch config.Environment {
	case "development", "test", "production":
	default:
		return fmt.Errorf("APP_ENV must be development, test, or production")
	}
	if config.IsProduction() {
		return config.ValidateProduction()
	}
	if config.TursoURL == "" || config.TursoToken == "" {
		return errors.New("TURSO_DATABASE_URL and TURSO_AUTH_TOKEN are required")
	}
	return nil
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
