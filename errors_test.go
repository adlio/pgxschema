package pgxschema

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

var (
	// ErrConnFailed indicates that the Conn() method failed (couldn't get a connection)
	ErrConnFailed = fmt.Errorf("connect failed")

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

// BadDB implements the interface for the *sql.DB Conn() method in a way that
// always fails
type BadDB struct{}

func (bd BadDB) Acquire(ctx context.Context) (*pgxpool.Conn, error) {
	return nil, ErrConnFailed
}

func TestApplyWithNilDBProvidesHelpfulError(t *testing.T) {
	migrator := NewMigrator()
	err := migrator.Apply(nil, testMigrations(t, "useless-ansi"))
	if !errors.Is(err, ErrNilDB) {
		t.Errorf("Expected %v, got %v", ErrNilDB, err)
	}
}

func TestApplyWithNoMigrations(t *testing.T) {
	withEachDB(t, func(db *pgxpool.Pool) {
		migrator := NewMigrator()
		err := migrator.Apply(db, []*Migration{})
		if err != nil {
			t.Errorf("Expected no error when running no migrations, got %s", err)
		}

	})
}
func TestApplyConnFailure(t *testing.T) {
	bd := BadDB{}
	withEachDB(t, func(db *pgxpool.Pool) {
		migrator := NewMigrator()
		err := migrator.Apply(bd, testMigrations(t, "useless-ansi"))
		if err != ErrConnFailed {
			t.Errorf("Expected %v, got %v", ErrConnFailed, err)
		}
	})
}

func TestLockFailure(t *testing.T) {
	bq := BadQueryer{}
	migrator := NewMigrator()
	err := migrator.lock(bq)
	expectErrorContains(t, err, "SELECT pg_advisory_lock")
}

func TestUnlockFailure(t *testing.T) {
	bq := BadQueryer{}
	migrator := NewMigrator()
	err := migrator.unlock(bq)
	expectErrorContains(t, err, "SELECT pg_advisory_unlock")
}

func TestComputeMigrationPlanFailure(t *testing.T) {
	bq := BadQueryer{}
	migrator := NewMigrator()
	_, err := migrator.computeMigrationPlan(bq, []*Migration{})
	expectErrorContains(t, err, "FAIL: SELECT id, checksum, execution_time_in_millis, applied_at")
}

func TestRunWithNilTransactionHasHelpfulError(t *testing.T) {
	migrator := NewMigrator()
	err := migrator.run(nil, testMigrations(t, "useless-ansi"))
	if err != ErrNilTx {
		t.Errorf("Expected %v, got %v", ErrNilTx, err)
	}
}

func expectErrorContains(t *testing.T, err error, contains string) {
	t.Helper()
	if err == nil {
		t.Errorf("Expected an error string containing '%s', got nil", contains)
	} else if !strings.Contains(err.Error(), contains) {
		t.Errorf("Expected an error string containing '%s', got '%s' instead", contains, err.Error())
	}
}
