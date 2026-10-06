// Package migrations bettet die SQL-Migrationen ins Binary ein.
package migrations

import "embed"

// FS enthält die Migrationsdateien (NNNN_name.sql, aufsteigend angewendet).
//
//go:embed *.sql
var FS embed.FS
