package pgxschema

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
)

var (
	// ErrBeginFailed indicates that the Begin() method failed (couldn't start Tx)
	ErrBeginFailed = fmt.Errorf("Begin Failed")

	// ErrPriorFailure indicates a failure happened earlier in the Migrator Apply()
	ErrPriorFailure = fmt.Errorf("Previous error")
)

// BadQueryer implements the Queryer interface, but fails on every call to
// Exec or Query. The error message will include the SQL statement to help
// verify the "right" failure occurred.
type BadQueryer struct{}

func (bq BadQueryer) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	return []byte{}, fmt.Errorf("FAIL: %s", strings.TrimSpace(sql))
}

func (bq BadQueryer) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return nil, fmt.Errorf("FAIL: %s", strings.TrimSpace(sql))
}

// BadTransactor implements the Connection interface, but fails to Begin any
// transactions.
type BadTransactor struct{}

func (bt BadTransactor) Begin(ctx context.Context) (pgx.Tx, error) {
	return nil, ErrBeginFailed
}

func (bt BadTransactor) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	return []byte{}, nil
}

func (bt BadTransactor) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return nil, nil
}
