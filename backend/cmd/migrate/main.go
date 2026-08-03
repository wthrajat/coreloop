package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"coreloop/backend/internal/config"
	"coreloop/backend/internal/database/migrate"
	"coreloop/backend/internal/database/tursohttp"
	projectmigrations "coreloop/migrations"
)

func main() {
	configuration := config.FromEnv()
	if err := configuration.ValidateRuntime(); err != nil {
		log.Fatal(err)
	}
	database, err := tursohttp.Open(configuration.TursoURL, configuration.TursoToken, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()
	migrations, err := migrate.Read(projectmigrations.Files)
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err := migrate.Apply(ctx, database, migrations); err != nil {
		log.Fatal(err)
	}
	_, _ = fmt.Fprintf(os.Stdout, "applied %d migrations\n", len(migrations))
}
