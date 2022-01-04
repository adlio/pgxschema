package pgxschema

import (
	"context"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
)

// Connection defines the interface for either a *pgxpool.Pool or a *pgx.Conn,
// both of which can start new transactions and execute queries.
type Connection interface {
	Transactor
	Queryer
}

// Queryer defines the interface for either a *pgxpool.Pool, a *pgx.Conn or a
// pgx.Tx, all of which can execute queries
//
type Queryer interface {
	Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
}

// Transactor defines the interface for either a *pgxpool.Pool or a *pgx.Conn,
// both of which can start new transactions.
type Transactor interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}
