package pgxschema

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

var (
	_ Transactor = &pgx.Conn{}
	_ Transactor = &pgxpool.Pool{}
	_ Queryer    = &pgx.Conn{}
	_ Queryer    = &pgxpool.Pool{}
)

// DefaultTableName defines the name of the database table which will
// hold the status of applied migrations
const DefaultTableName = "schema_migrations"

// ErrNilDB is thrown when the database pointer is nil
var ErrNilDB = errors.New("DB pointer is nil")

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

// transaction wraps the supplied function in a transaction with the supplied
// database connecion
//
func (m *Migrator) transaction(db Transactor, f func(context.Context, pgx.Tx) error) (err error) {
	if db == nil {
		return ErrNilDB
	}
	tx, err := db.Begin(m.ctx)
	if err != nil {
		return
	}

	defer func() {
		if p := recover(); p != nil {
			switch p := p.(type) {
			case error:
				err = p
			default:
				err = fmt.Errorf("%s", p)
			}
		}
		if err != nil {
			_ = tx.Rollback(m.ctx)
			return
		}
		err = tx.Commit(m.ctx)
	}()

	return f(m.ctx, tx)
}
