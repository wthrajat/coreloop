package migrate

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSplitStatementsPreservesQuotedSemicolon(t *testing.T) {
	statements := SplitStatements("-- comment\nCREATE TABLE x (v TEXT); INSERT INTO x VALUES ('a;b');")
	if len(statements) != 2 {
		t.Fatalf("got %d statements: %#v", len(statements), statements)
	}
	if statements[1] != "INSERT INTO x VALUES ('a;b')" {
		t.Fatalf("unexpected statement %q", statements[1])
	}
}

func TestSplitStatementsPreservesTriggerBody(t *testing.T) {
	input := `CREATE TRIGGER record_failure
		AFTER UPDATE ON jobs
		BEGIN
			INSERT INTO events(value) VALUES (
				CASE WHEN NEW.value = 'END; still text' THEN 'first' ELSE 'second' END
			);
		END;
		INSERT INTO schema_migrations(version) VALUES (10);`
	statements := SplitStatements(input)
	if len(statements) != 2 {
		t.Fatalf("got %d statements: %#v", len(statements), statements)
	}
	trigger := statements[0]
	if !strings.HasPrefix(trigger, "CREATE TRIGGER") ||
		!strings.Contains(trigger, ");\n\t\tEND") {
		t.Fatalf("trigger body was split or changed: %q", trigger)
	}
	if statements[1] != "INSERT INTO schema_migrations(version) VALUES (10)" {
		t.Fatalf("unexpected trailing statement %q", statements[1])
	}
}

func TestSplitStatementsPreservesTemporaryTriggerBody(t *testing.T) {
	input := `CREATE TEMP TRIGGER record_change AFTER UPDATE ON jobs BEGIN
		INSERT INTO events(value) VALUES (NEW.value);
	END; SELECT 1;`
	statements := SplitStatements(input)
	if len(statements) != 2 || !strings.Contains(statements[0], "VALUES (NEW.value);\n\tEND") {
		t.Fatalf("temporary trigger split incorrectly: %#v", statements)
	}
}

func TestSplitStatementsPreservesJobFailureDiagnosticsMigration(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current test file")
	}
	projectRoot := filepath.Clean(filepath.Join(
		filepath.Dir(currentFile),
		"..",
		"..",
		"..",
		"..",
	))
	migration, err := os.ReadFile(filepath.Join(
		projectRoot,
		"migrations",
		"0010_job_failure_diagnostics.sql",
	))
	if err != nil {
		t.Fatal(err)
	}
	statements := SplitStatements(string(migration))
	if len(statements) != 5 {
		t.Fatalf("migration 10 split into %d statements: %#v", len(statements), statements)
	}
	trigger := statements[3]
	if !strings.HasPrefix(trigger, "CREATE TRIGGER job_queue_record_failure") ||
		!strings.HasSuffix(trigger, "END") ||
		!strings.Contains(trigger, "NEW.last_error_at\n    );") {
		t.Fatalf("migration 10 trigger was not preserved: %q", trigger)
	}
}
