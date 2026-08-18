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

func TestSourceHealthMigrationAddsSafeOperationalDiagnostics(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current test file")
	}
	projectRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
	migration, err := os.ReadFile(filepath.Join(projectRoot, "migrations", "0011_source_health.sql"))
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ToLower(string(migration))
	for _, required := range []string{
		"last_poll_state",
		"last_success_at",
		"last_error_code",
		"last_error_summary",
		"last_item_count",
		"radar-reindex:deterministic-editorial-v4",
		"source_position <= 25",
		"values (11, 'source_health')",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("source-health migration is missing %q", required)
		}
	}
}

func TestRadarSourceExpansionAddsEveryReviewedSource(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current test file")
	}
	projectRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
	migrationPath := filepath.Join(projectRoot, "migrations", "0012_expand_radar_sources.sql")
	migration, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatal(err)
	}
	migrationText := strings.ToLower(string(migration))
	requiredSourceIDs := []string{
		"source_uber_engineering", "source_spotify_engineering", "source_discord_blog",
		"source_nvidia_technical_blog", "source_mistral_news", "source_google_project_zero",
		"source_google_security_blog", "source_chrome_developers", "source_docker_blog",
		"source_gitlab_releases", "source_stripe_engineering", "source_cloudflare_research",
		"source_arxiv_distributed", "source_usenix_publications", "source_acm_cacm",
		"source_trail_of_bits", "source_ncc_research", "source_msrc_blog",
		"source_duckdb_blog", "source_clickhouse_blog", "source_cockroach_blog",
		"source_sqlite_releases", "source_typescript_blog", "source_dotnet_blog",
		"source_inside_java", "source_kotlin_blog", "source_deno_blog", "source_bun_blog",
		"source_hashicorp_blog", "source_grafana_blog", "source_datadog_blog",
		"source_fly_blog", "source_tailscale_blog", "source_meta_ai",
		"source_google_developers_blog", "source_huggingface_transformers",
		"source_huggingface_hub", "source_huggingface_tgi", "source_airbnb_engineering",
		"source_doordash_engineering", "source_shopify_engineering", "source_figma_engineering",
		"source_ietf_blog", "source_whatwg_blog", "source_w3c_news",
	}
	for _, sourceID := range requiredSourceIDs {
		if !strings.Contains(migrationText, "'"+sourceID+"'") {
			t.Errorf("Radar source expansion is missing %q", sourceID)
		}
	}
	for _, required := range []string{
		`"adapter":"html_listing"`,
		`"adapter":"github_releases"`,
		`"allowed_hosts":["blog.cloudflare.com"]`,
		"values (12, 'expand_radar_sources')",
	} {
		if !strings.Contains(migrationText, required) {
			t.Errorf("Radar source expansion is missing %q", required)
		}
	}

	var script strings.Builder
	for version := 1; version <= 12; version++ {
		matches, globErr := filepath.Glob(filepath.Join(
			projectRoot,
			"migrations",
			fmt.Sprintf("%04d_*.sql", version),
		))
		if globErr != nil || len(matches) != 1 {
			t.Fatalf("resolve migration %d: %v, matches=%v", version, globErr, matches)
		}
		script.WriteString(".read " + matches[0] + "\n")
	}
	script.WriteString(`SELECT COUNT(*) || ',' ||
		SUM(json_extract(adapter_config_json,'$.adapter')='html_listing') || ',' ||
		SUM(enabled=1) || ',' ||
		SUM(canonical_url NOT LIKE 'https://%') || ',' ||
		(SELECT MAX(version) FROM schema_migrations)
		FROM sources;` + "\n")
	command := exec.Command("sqlite3", ":memory:")
	command.Stdin = strings.NewReader(script.String())
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("run Radar source expansion migration: %v: %s", err, output)
	}
	if strings.TrimSpace(string(output)) != "86,10,86,0,12" {
		t.Fatalf("expanded Radar catalogue = %s", output)
	}
}

func TestSourceHealthMigrationQueuesBoundedFreshRadarReindex(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current test file")
	}
	projectRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
	var script strings.Builder
	for version := 1; version <= 10; version++ {
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
	script.WriteString(`WITH RECURSIVE numbers(value) AS (
		SELECT 1 UNION ALL SELECT value+1 FROM numbers WHERE value<26
	) INSERT INTO source_items(
		id,source_id,canonical_url,normalized_url,title,published_at,retrieved_at,
		content_hash,evidence_json
	) SELECT
		'reindex-'||value,CASE WHEN value<=13 THEN 'source_hacker_news' ELSE 'source_stacker_news' END,'https://example.co/'||value,
		'https://example.co/'||value,'Fresh item '||value,
		strftime('%Y-%m-%dT%H:%M:%fZ','now'),strftime('%Y-%m-%dT%H:%M:%fZ','now'),
		'hash-'||value,'{}' FROM numbers;` + "\n")
	script.WriteString(".read " + filepath.Join(
		projectRoot,
		"migrations",
		"0011_source_health.sql",
	) + "\n")
	script.WriteString(`SELECT COUNT(*)||','||SUM(json_array_length(json_extract(payload_json,'$.source_item_ids'))) FROM job_queue WHERE idempotency_key LIKE 'radar-reindex:deterministic-editorial-v4:%';` + "\n")

	command := exec.Command("sqlite3", ":memory:")
	command.Stdin = strings.NewReader(script.String())
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("run populated source-health migration: %v: %s", err, output)
	}
	if strings.TrimSpace(string(output)) != "2,26" {
		t.Fatalf("Radar reindex migration state = %s, want two jobs with 26 source-balanced items", output)
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

func TestJobFailureDiagnosticsMigrationRecordsEveryAttemptAtomically(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current test file")
	}
	projectRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
	matches, err := filepath.Glob(filepath.Join(
		projectRoot,
		"migrations",
		"[0-9][0-9][0-9][0-9]_*.sql",
	))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(matches)
	var script strings.Builder
	for _, migration := range matches {
		script.WriteString(".read " + migration + "\n")
	}
	script.WriteString(`INSERT INTO job_queue(id,job_type,state,due_at,attempt_count,max_attempts,idempotency_key,payload_json,lease_owner,lease_expires_at) VALUES('job-diagnostic','generate_lesson','leased','2026-08-05T10:00:00Z',1,5,'diagnostic','{}','worker','2026-08-05T10:05:00Z');` + "\n")
	script.WriteString(`UPDATE job_queue SET state='queued',last_error_code='ai_invalid_output',last_error_summary='Groq returned invalid output.',last_error_at='2026-08-05T10:01:00Z' WHERE id='job-diagnostic';` + "\n")
	script.WriteString(`UPDATE job_queue SET state='leased',attempt_count=2,last_error_at='2026-08-05T10:01:00Z' WHERE id='job-diagnostic';` + "\n")
	script.WriteString(`UPDATE job_queue SET state='failed',last_error_code='execution_timeout',last_error_summary='The job exceeded its execution deadline.',last_error_at='2026-08-05T10:02:00Z' WHERE id='job-diagnostic';` + "\n")
	script.WriteString(`SELECT COUNT(*) || ',' || MIN(attempt_count) || ',' || MAX(attempt_count) || ',' || (SELECT last_error_summary FROM job_queue WHERE id='job-diagnostic') FROM job_failure_events WHERE job_id='job-diagnostic';` + "\n")

	command := exec.Command("sqlite3", ":memory:")
	command.Stdin = strings.NewReader(script.String())
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("run failure diagnostics migration: %v: %s", err, output)
	}
	want := "2,1,2,The job exceeded its execution deadline."
	if strings.TrimSpace(string(output)) != want {
		t.Fatalf("failure diagnostics = %s, want %s", output, want)
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
