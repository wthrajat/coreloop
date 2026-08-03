package migrations

import "embed"

// Files contains the ordered production migrations.
//
//go:embed *.sql
var Files embed.FS
