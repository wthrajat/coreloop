package store

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
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

func TestAccountDeletionCascadesPrivateRows(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current test file")
	}
	projectRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
	matches, err := filepath.Glob(filepath.Join(projectRoot, "migrations", "[0-9][0-9][0-9][0-9]_*.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(matches)
	var script strings.Builder
	for _, migration := range matches {
		script.WriteString(".read " + migration + "\n")
	}
	script.WriteString("INSERT INTO users(id,telegram_subject) VALUES('cascade-user','subject');\n")
	script.WriteString("INSERT INTO learning_profiles(id,user_id) VALUES('profile','cascade-user');\n")
	script.WriteString("DELETE FROM users WHERE id='cascade-user';\n")
	script.WriteString("SELECT COUNT(*) FROM learning_profiles WHERE id='profile';\n")
	command := exec.Command("sqlite3", ":memory:")
	command.Stdin = strings.NewReader(script.String())
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("run cascade integration test: %v: %s", err, output)
	}
	if strings.TrimSpace(string(output)) != "0" {
		t.Fatalf("private profile survived account deletion: %s", output)
	}
}

func TestRequiredSchemaVersionMatchesLatestMigration(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current test file")
	}
	projectRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
	matches, err := filepath.Glob(filepath.Join(projectRoot, "migrations", "[0-9][0-9][0-9][0-9]_*.sql"))
	if err != nil || len(matches) == 0 {
		t.Fatalf("list migrations: %v", err)
	}
	sort.Strings(matches)
	latestName := filepath.Base(matches[len(matches)-1])
	latestVersion, err := strconv.Atoi(latestName[:4])
	if err != nil {
		t.Fatal(err)
	}
	if RequiredSchemaVersion != latestVersion {
		t.Fatalf("required schema version = %d, latest migration = %d", RequiredSchemaVersion, latestVersion)
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

func TestPerformanceMigrationAddsIndexesAndRecallAttachment(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current test file")
	}
	projectRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
	migration, err := os.ReadFile(filepath.Join(
		projectRoot,
		"migrations",
		"0007_performance_and_integrity.sql",
	))
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ToLower(string(migration))
	for _, required := range []string{
		"job_queue_user_state_sequence_idx",
		"radar_candidates_cleanup_idx",
		"add column recall_review_id",
		"lesson_assignments_recall_review_idx",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("performance migration is missing %q", required)
		}
	}
}
