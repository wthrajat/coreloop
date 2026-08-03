package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"coreloop/backend/app"
)

func main() {
	address := os.Getenv("CORELOOP_API_ADDRESS")
	if address == "" {
		address = ":8080"
	}

	router := http.NewServeMux()
	router.Handle("/api/app", app.NewHTTPHandler("local"))
	router.Handle("/api/app/", app.NewHTTPHandler("local"))
	router.Handle("/api/jobs/", app.NewJobsHandler())
	router.Handle("/api/telegram/", app.NewTelegramHandler())
	server := &http.Server{
		Addr:              address,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("coreloop API listening on %s", address)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
