package store

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInitialMigrationContainsRequiredTables(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current test file")
	}

	projectRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
	migrationPath := filepath.Join(projectRoot, "migrations", "0001_initial.sql")
	migration, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("read initial migration: %v", err)
	}

	migrationText := strings.ToLower(string(migration))
	if !strings.Contains(migrationText, "pragma foreign_keys = on") {
		t.Fatal("initial migration must enable foreign-key enforcement")
	}

	for _, tableName := range RequiredInitialTables {
		expectedDefinition := "create table " + tableName
		if !strings.Contains(migrationText, expectedDefinition) {
			t.Errorf("initial migration is missing table %q", tableName)
		}
	}
}
