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
	"github.com/pashagolub/pgxmock"
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
func TestApplyBeginFailure(t *testing.T) {
	mock, err := pgxmock.NewConn()
	if err != nil {
		t.Error(err)
	}
	mock.ExpectExec("^SELECT pg_advisory_lock").WillReturnResult(pgconn.CommandTag{})
	mock.ExpectBegin().WillReturnError(fmt.Errorf("Begin Failed"))
	migrator := NewMigrator()
	err = migrator.Apply(mock, testMigrations(t, "useless-ansi"))
	expectErrorContains(t, err, "Begin Failed")
}

func TestApplyLockFailure(t *testing.T) {
	mock, err := pgxmock.NewConn()
	if err != nil {
		t.Error(err)
	}
	mock.ExpectExec("^SELECT pg_advisory_lock").WillReturnError(fmt.Errorf("Lock Failed"))
	err = NewMigrator().Apply(mock, testMigrations(t, "useless-ansi"))
	expectErrorContains(t, err, "Lock Failed")
}

func TestApplyCreateMigrationsTableFailure(t *testing.T) {
	mock, err := pgxmock.NewConn()
	if err != nil {
		t.Error(err)
	}
	mock.ExpectExec("^SELECT pg_advisory_lock").WillReturnResult(pgconn.CommandTag{})
	mock.ExpectBegin()
	mock.ExpectQuery("^CREATE TABLE").WillReturnError(fmt.Errorf("Create Migrations Table Failed"))
	err = NewMigrator().Apply(mock, testMigrations(t, "useless-ansi"))
	expectErrorContains(t, err, "Create Migrations Table Failed")
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

func TestRunWithComputePlanFailHasHelpfulError(t *testing.T) {
	bq := BadQueryer{}
	err := NewMigrator().run(bq, testMigrations(t, "useless-ansi"))
	expectErrorContains(t, err, "SELECT id, checksum")
}

func expectErrorContains(t *testing.T, err error, contains string) {
	t.Helper()
	if err == nil {
		t.Errorf("Expected an error string containing '%s', got nil", contains)
	} else if !strings.Contains(err.Error(), contains) {
		t.Errorf("Expected an error string containing '%s', got '%s' instead", contains, err.Error())
	}
}
