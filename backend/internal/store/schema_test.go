package store

import (
	"fmt"
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
	script.WriteString("INSERT INTO invites(id,token_hash,expires_at,consumed_at,consumed_by_user_id) VALUES('consumed-invite','token-hash','2099-01-01T00:00:00Z','2026-08-05T00:00:00Z','cascade-user');\n")
	script.WriteString("INSERT INTO learning_profiles(id,user_id) VALUES('profile','cascade-user');\n")
	script.WriteString("BEGIN;\n")
	script.WriteString("DELETE FROM invites WHERE consumed_by_user_id='cascade-user';\n")
	script.WriteString("DELETE FROM users WHERE id='cascade-user';\n")
	script.WriteString("COMMIT;\n")
	script.WriteString("SELECT COUNT(*) || ',' || (SELECT COUNT(*) FROM invites WHERE id='consumed-invite') FROM learning_profiles WHERE id='profile';\n")
	command := exec.Command("sqlite3", ":memory:")
	command.Stdin = strings.NewReader(script.String())
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("run cascade integration test: %v: %s", err, output)
	}
	if strings.TrimSpace(string(output)) != "0,0" {
		t.Fatalf("private account data survived deletion: %s", output)
	}
}

func TestSecurityMigrationBindsOIDCFlowsToTheStartingBrowser(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current test file")
	}
	projectRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
	migration, err := os.ReadFile(filepath.Join(projectRoot, "migrations", "0008_security_hardening.sql"))
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ToLower(string(migration))
	for _, required := range []string{
		"browser_binding_hash",
		"values (8, 'security_hardening')",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("security migration is missing %q", required)
		}
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

func TestDeliveryQualityMigrationRegistersStrictPrompt(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current test file")
	}
	projectRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
	migration, err := os.ReadFile(filepath.Join(
		projectRoot,
		"migrations",
		"0009_delivery_quality.sql",
	))
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ToLower(string(migration))
	for _, required := range []string{
		"superseded_duplicate_source_poll",
		"job_queue_one_active_source_poll_idx",
		"prompt_lesson_v4",
		"lesson-v4",
		"lesson-draft-v1",
		"compiler-v4",
		"values (9, 'delivery_quality')",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("delivery quality migration is missing %q", required)
		}
	}
}

func TestDeliveryQualityMigrationCollapsesDuplicateSourcePolls(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current test file")
	}
	projectRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
	var script strings.Builder
	for version := 1; version <= 8; version++ {
		matches, err := filepath.Glob(filepath.Join(
			projectRoot,
			"migrations",
			fmt.Sprintf("%04d_*.sql", version),
		))
		if err != nil || len(matches) != 1 {
			t.Fatalf("resolve migration %d: %v, matches=%v", version, err, matches)
		}
		script.WriteString(".read " + matches[0] + "\n")
	}
	for _, id := range []string{"poll-1", "poll-2", "poll-3", "poll-4"} {
		script.WriteString(fmt.Sprintf(
			`INSERT INTO job_queue(id,job_type,due_at,idempotency_key,payload_json) VALUES('%s','ingest_source','2026-08-05T10:00:00Z','%s','{"source_id":"source_openai"}');`+"\n",
			id,
			id,
		))
	}
	script.WriteString(`UPDATE job_queue SET state='leased',lease_owner='worker',lease_expires_at='2026-08-05T10:05:00Z' WHERE id IN ('poll-3','poll-4');` + "\n")
	script.WriteString(".read " + filepath.Join(
		projectRoot,
		"migrations",
		"0009_delivery_quality.sql",
	) + "\n")
	script.WriteString(`INSERT INTO job_queue(id,job_type,due_at,idempotency_key,payload_json) VALUES('poll-5','ingest_source','2026-08-05T10:00:00Z','poll-5','{"source_id":"source_openai"}') ON CONFLICT DO NOTHING;` + "\n")
	script.WriteString(`SELECT SUM(state IN ('queued','leased')) || ',' || SUM(state='cancelled') FROM job_queue WHERE job_type='ingest_source';` + "\n")

	command := exec.Command("sqlite3", ":memory:")
	command.Stdin = strings.NewReader(script.String())
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("run populated delivery-quality migration: %v: %s", err, output)
	}
	if strings.TrimSpace(string(output)) != "1,3" {
		t.Fatalf("source poll migration state = %s, want 1 active and 3 cancelled", output)
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
