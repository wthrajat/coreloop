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

func TestRadarQualityMigrationContainsDurableFallbackAndFrequencyState(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current test file")
	}
	projectRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
	migration, err := os.ReadFile(filepath.Join(projectRoot, "migrations", "0004_radar_quality.sql"))
	if err != nil {
		t.Fatalf("read Radar quality migration: %v", err)
	}
	text := strings.ToLower(string(migration))
	for _, required := range []string{
		"radar_items_per_day",
		"radar_weekends_enabled",
		"set normalized_url=canonical_url",
		"community_signals_available",
		"create table radar_deliveries",
		"create table radar_delivery_parts",
		"create table radar_daily_usage",
		"create table radar_enrichments",
		"source_hacker_news",
		"source_openai_news",
		"source_stacker_news",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("Radar quality migration is missing %q", required)
		}
	}
}

func TestLessonClarityMigrationRegistersVersionedPrompt(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current test file")
	}
	projectRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
	migration, err := os.ReadFile(filepath.Join(projectRoot, "migrations", "0005_lesson_clarity.sql"))
	if err != nil {
		t.Fatalf("read lesson clarity migration: %v", err)
	}
	text := strings.ToLower(string(migration))
	for _, required := range []string{
		"prompt_lesson_v2",
		"lesson-v2",
		"lesson-draft-v1",
		"compiler-v2",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("lesson clarity migration is missing %q", required)
		}
	}
}

func TestGroundedSourceMigrationRegistersVersionedPrompt(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current test file")
	}
	projectRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
	migration, err := os.ReadFile(filepath.Join(
		projectRoot,
		"migrations",
		"0006_grounded_lesson_sources.sql",
	))
	if err != nil {
		t.Fatalf("read grounded source migration: %v", err)
	}
	text := strings.ToLower(string(migration))
	for _, required := range []string{
		"prompt_lesson_v3",
		"lesson-v3",
		"lesson-draft-v1",
		"compiler-v3",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("grounded source migration is missing %q", required)
		}
	}
}
