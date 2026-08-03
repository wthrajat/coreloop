package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

type Migration struct {
	Version int
	Name    string
	SQL     string
}

func Read(files fs.FS) ([]Migration, error) {
	entries, err := fs.Glob(files, "*.sql")
	if err != nil {
		return nil, fmt.Errorf("list migrations: %w", err)
	}
	sort.Strings(entries)
	migrations := make([]Migration, 0, len(entries))
	for _, name := range entries {
		parts := strings.SplitN(name, "_", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid migration name %q", name)
		}
		version, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, fmt.Errorf("invalid migration version %q: %w", name, err)
		}
		content, err := fs.ReadFile(files, name)
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", name, err)
		}
		migrations = append(migrations, Migration{Version: version, Name: name, SQL: string(content)})
	}
	return migrations, nil
}

func Apply(ctx context.Context, database *sql.DB, migrations []Migration) error {
	if _, err := database.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
	)`); err != nil {
		return fmt.Errorf("prepare migration ledger: %w", err)
	}
	for _, migration := range migrations {
		var applied int
		err := database.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE version = ?", migration.Version).Scan(&applied)
		if err != nil {
			return fmt.Errorf("check migration %d: %w", migration.Version, err)
		}
		if applied > 0 {
			continue
		}
		transaction, err := database.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", migration.Version, err)
		}
		for _, statement := range SplitStatements(migration.SQL) {
			normalized := strings.ToUpper(strings.TrimSpace(statement))
			if normalized == "BEGIN IMMEDIATE" || normalized == "BEGIN" || normalized == "COMMIT" ||
				strings.HasPrefix(normalized, "PRAGMA FOREIGN_KEYS") ||
				strings.HasPrefix(normalized, "INSERT INTO SCHEMA_MIGRATIONS") ||
				strings.HasPrefix(normalized, "CREATE TABLE SCHEMA_MIGRATIONS") {
				continue
			}
			if _, err := transaction.ExecContext(ctx, statement); err != nil {
				_ = transaction.Rollback()
				return fmt.Errorf("apply migration %d statement %.80q: %w", migration.Version, statement, err)
			}
		}
		if _, err := transaction.ExecContext(ctx, "INSERT INTO schema_migrations (version, name) VALUES (?, ?)", migration.Version, migration.Name); err != nil {
			_ = transaction.Rollback()
			return fmt.Errorf("record migration %d: %w", migration.Version, err)
		}
		if err := transaction.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", migration.Version, err)
		}
	}
	return nil
}

// SplitStatements handles SQLite strings and comments without accepting a
// second SQL parser as a production dependency.
func SplitStatements(input string) []string {
	var statements []string
	var builder strings.Builder
	var quote rune
	lineComment := false
	blockComment := false
	runes := []rune(input)
	for index := 0; index < len(runes); index++ {
		current := runes[index]
		next := rune(0)
		if index+1 < len(runes) {
			next = runes[index+1]
		}
		if lineComment {
			if current == '\n' {
				lineComment = false
				builder.WriteRune(current)
			}
			continue
		}
		if blockComment {
			if current == '*' && next == '/' {
				blockComment = false
				index++
			}
			continue
		}
		if quote == 0 && current == '-' && next == '-' {
			lineComment = true
			index++
			continue
		}
		if quote == 0 && current == '/' && next == '*' {
			blockComment = true
			index++
			continue
		}
		if current == '\'' || current == '"' {
			if quote == 0 {
				quote = current
			} else if quote == current {
				if next == current {
					builder.WriteRune(current)
					builder.WriteRune(next)
					index++
					continue
				}
				quote = 0
			}
		}
		if current == ';' && quote == 0 {
			statement := strings.TrimSpace(builder.String())
			if statement != "" {
				statements = append(statements, statement)
			}
			builder.Reset()
			continue
		}
		if quote != 0 || !unicode.IsSpace(current) || builder.Len() > 0 {
			builder.WriteRune(current)
		}
	}
	if statement := strings.TrimSpace(builder.String()); statement != "" {
		statements = append(statements, statement)
	}
	return statements
}
