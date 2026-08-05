package config

import (
	"strings"
	"testing"
)

func TestFromEnvInfersVercelProduction(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("VERCEL_ENV", "production")

	configuration := FromEnv()
	if !configuration.IsProduction() || configuration.Environment != "production" {
		t.Fatalf("production was not inferred: %#v", configuration)
	}
}

func TestProductionRejectsOriginWithPath(t *testing.T) {
	configuration := validProductionConfig()
	configuration.AppOrigin = "https://coreloop.example/api"
	if err := configuration.ValidateProduction(); err == nil || !strings.Contains(err.Error(), "without path") {
		t.Fatalf("origin validation error = %v", err)
	}
}

func TestProductionRejectsVercelEnvironmentMismatch(t *testing.T) {
	configuration := validProductionConfig()
	configuration.VercelEnvironment = "preview"
	if err := configuration.ValidateRuntime(); err == nil || !strings.Contains(err.Error(), "VERCEL_ENV") {
		t.Fatalf("environment validation error = %v", err)
	}
}

func TestRuntimeRejectsUnknownApplicationEnvironment(t *testing.T) {
	configuration := Config{Environment: "prod", TursoURL: "libsql://database.turso.io", TursoToken: "token"}
	if err := configuration.ValidateRuntime(); err == nil || !strings.Contains(err.Error(), "APP_ENV") {
		t.Fatalf("environment validation error = %v", err)
	}
}

func TestSecureCookiesFollowHTTPSOrigin(t *testing.T) {
	if !(Config{AppOrigin: "https://coreloop.example"}).SecureCookies() {
		t.Fatal("HTTPS origin must use secure cookies")
	}
	if (Config{AppOrigin: "http://localhost:3000"}).SecureCookies() {
		t.Fatal("local HTTP origin cannot use secure cookies")
	}
}

func validProductionConfig() Config {
	return Config{
		Environment: "production", AppOrigin: "https://coreloop.example",
		TursoURL: "libsql://database.turso.io", TursoToken: "token",
		TelegramClientID: "client", TelegramClientSecret: "client-secret",
		TelegramBotToken: "bot-token", TelegramWebhookSecret: strings.Repeat("w", 32),
		OwnerTelegramSubject: "123", QStashCurrentSigningKey: "current",
		QStashNextSigningKey: "next", QStashToken: "token",
		GroqAPIKey: "groq", SessionSecret: strings.Repeat("s", 32),
		TimeZone: defaultTimeZone,
	}
}
