package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"coreloop/backend/internal/config"
	"coreloop/backend/internal/database/tursohttp"
	"coreloop/backend/internal/ids"
	"coreloop/backend/internal/securehash"
	"coreloop/backend/internal/store"
	"coreloop/backend/internal/telegram"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	configuration := config.FromEnv()
	switch os.Args[1] {
	case "invite":
		createInvite(configuration, os.Args[2:])
	case "telegram-webhook":
		setWebhook(configuration)
	default:
		usage()
	}
}
func createInvite(configuration config.Config, arguments []string) {
	flags := flag.NewFlagSet("invite", flag.ExitOnError)
	ttl := flags.Duration("ttl", 7*24*time.Hour, "invite lifetime")
	_ = flags.Parse(arguments)
	database, err := tursohttp.Open(configuration.TursoURL, configuration.TursoToken, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()
	token, err := ids.Token(32)
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	invite, err := store.New(database).CreateInvite(ctx, securehash.Keyed(token, configuration.SessionSecret), "", time.Now().Add(*ttl))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("invite %s expires %s\n%s/invite/%s\n", invite.ID, invite.ExpiresAt.Format(time.RFC3339), configuration.AppOrigin, token)
}
func setWebhook(configuration config.Config) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	endpoint := configuration.AppOrigin + "/api/telegram/webhook"
	if err := telegram.New(configuration.TelegramBotToken, nil).SetWebhook(ctx, endpoint, configuration.TelegramWebhookSecret); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Telegram webhook configured: %s\n", endpoint)
}
func usage() {
	fmt.Fprintln(os.Stderr, "usage: go run ./backend/cmd/admin <invite|telegram-webhook>")
	os.Exit(2)
}
