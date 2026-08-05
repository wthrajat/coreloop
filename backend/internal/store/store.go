package store

import (
	"context"
	"database/sql"
	"fmt"
)

type Store struct {
	database *sql.DB
}

const RequiredSchemaVersion = 8

func New(database *sql.DB) *Store {
	return &Store{database: database}
}

func (store *Store) Ping(ctx context.Context) error {
	if err := store.database.PingContext(ctx); err != nil {
		return err
	}
	var version int
	if err := store.database.QueryRowContext(ctx, "SELECT COALESCE(MAX(version),0) FROM schema_migrations").Scan(&version); err != nil {
		return err
	}
	if version < RequiredSchemaVersion {
		return fmt.Errorf("database schema is at version %d; version %d is required", version, RequiredSchemaVersion)
	}
	return nil
}

func (store *Store) Database() *sql.DB { return store.database }
