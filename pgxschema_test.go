package pgxschema

import (
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Interface verification that *sql.DB satisfies our DB interface
var (
	_ DB = &pgxpool.Pool{}
)

// Interface verification that pgx.Conn and pgxpool.Pool both satisfy our
// Connection interface
var (
	_ Connection = &pgx.Conn{}
	_ Connection = &pgxpool.Pool{}
)

// Interface verification that pgx.Conn and pgxpool.Pool both satisfy our
// Transactor interface
var (
	_ Transactor = &pgx.Conn{}
	_ Transactor = &pgxpool.Pool{}
)

// Interface verification that pgx.Conn, pgxpool.Pool and pgx.Tx all support
// our Queryer interface.
var (
	_ Queryer = &pgx.Conn{}
	_ Queryer = &pgxpool.Pool{}
	_ Queryer = pgx.Tx(nil)
)

// TestDBs holds all of the specific database instances against which tests
// will run.
var TestDBs = map[string]*TestDB{
	"postgres:11": {
		DockerRepo: "postgres",
		DockerTag:  "11",
	},
	"postgres:12": {
		DockerRepo: "postgres",
		DockerTag:  "12",
	},
	"postgres:latest": {
		DockerRepo: "postgres",
		DockerTag:  "latest",
	},
}
