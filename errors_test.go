package pgxschema

import (
	"context"
	"fmt"
	"strings"
	"testing"

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
	m := makeTestMigrator()
	_, err := m.computeMigrationPlan(bq, []*Migration{})
	expectedContents := "FAIL: SELECT id, checksum, execution_time_in_millis, applied_at"
	if err == nil || !strings.Contains(err.Error(), expectedContents) {
		t.Errorf("Expected error msg with '%s'. Got '%v'.", expectedContents, err)
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
