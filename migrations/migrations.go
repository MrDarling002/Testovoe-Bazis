// Package migrations embeds the SQL schema migrations that are applied
// automatically at application startup via golang-migrate.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
