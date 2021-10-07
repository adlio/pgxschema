// Package pgxschema provides tools to manage database schema changes
// ("migrations") as embedded functionality inside another application
// which is using jackc/pgx as its PostgreSQL driver.
//
// Basic usage instructions involve creating a pgxschema.Migrator via the
// pgxschema.NewMigrator() function, and then passing your pgx.Conn or
// pgxpool.Pool to its .Apply() method.
//
// See the package's README.md file for more usage instructions.
//
package pgxschema
