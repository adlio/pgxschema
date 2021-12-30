package pgxschema

import (
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
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
	"postgres11": {
		DockerRepo: "postgres",
		DockerTag:  "11",
	},
	"postgres13": {
		DockerRepo: "postgres",
		DockerTag:  "13",
	},
}
