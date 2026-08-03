package migrate

import "testing"

func TestSplitStatementsPreservesQuotedSemicolon(t *testing.T) {
	statements := SplitStatements("-- comment\nCREATE TABLE x (v TEXT); INSERT INTO x VALUES ('a;b');")
	if len(statements) != 2 {
		t.Fatalf("got %d statements: %#v", len(statements), statements)
	}
	if statements[1] != "INSERT INTO x VALUES ('a;b')" {
		t.Fatalf("unexpected statement %q", statements[1])
	}
}
