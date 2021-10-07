package pgxschema

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)

func TestApplyWithNilDBProvidesHelpfulError(t *testing.T) {
	m := NewMigrator()
	err := m.Apply(nil, []*Migration{
		{
			ID:     "2019-01-01 Test",
			Script: "CREATE TABLE fake_table (id INTEGER)",
		},
	})
	if !errors.Is(err, ErrNilDB) {
		t.Errorf("Expected %v, got %v", ErrNilDB, err)
	}
}
func TestGetAppliedMigrationsErrorsWhenNoneExist(t *testing.T) {
	withLatestDB(t, func(db *pgxpool.Pool) {
		migrator := makeTestMigrator()
		migrations, err := migrator.GetAppliedMigrations(db)
		if err == nil {

			t.Error("Expected an error. Got  none.")
		}
		if len(migrations) > 0 {
			t.Error("Expected empty list of applied migrations")
		}
	})
}

func TestApplyMultistatementMigrations(t *testing.T) {
	withEachDB(t, func(db *pgxpool.Pool) {
		migrator := makeTestMigrator()
		migrationSet1 := []*Migration{
			{
				ID: "2019-09-23 Create Artists and Albums",
				Script: `
		CREATE TABLE artists (
			id SERIAL PRIMARY KEY,
			name CHARACTER VARYING (255) NOT NULL DEFAULT ''
		);
		CREATE UNIQUE INDEX idx_artists_name ON artists (name);
		CREATE TABLE albums (
			id SERIAL PRIMARY KEY,
			title CHARACTER VARYING (255) NOT NULL DEFAULT '',
			artist_id INTEGER NOT NULL REFERENCES artists(id)
		);
		`,
			},
		}
		err := migrator.Apply(db, migrationSet1)
		if err != nil {
			t.Error(err)
		}

		err = migrator.Apply(db, migrationSet1)
		if err != nil {
			t.Error(err)
		}

		migrationSet2 := []*Migration{
			{
				ID: "2019-09-24 Create Tracks",
				Script: `
		CREATE TABLE tracks (
			id SERIAL PRIMARY KEY,
			name CHARACTER VARYING (255) NOT NULL DEFAULT '',
			artist_id INTEGER NOT NULL REFERENCES artists(id),
			album_id INTEGER NOT NULL REFERENCES albums(id)
		);`,
			},
		}
		err = migrator.Apply(db, migrationSet2)
		if err != nil {
			t.Error(err)
		}
	})
}

func TestCommitOrRollbackRecoversErrorPanic(t *testing.T) {
	m := makeTestMigrator()
	defer func() {
		if m.err != ErrPriorFailure {
			t.Errorf("Expected error '%v'. Got '%v'.", ErrPriorFailure, m.err)
		}
	}()
	defer m.commitOrRollback(nil)
	panic(ErrPriorFailure)
}

func TestCommitOrRollbackRecoversNakedPanic(t *testing.T) {
	m := makeTestMigrator()
	defer func() {
		expectedContents := "Runtime error"
		if m.err.Error() != expectedContents {
			t.Errorf("Expected error '%v'. Got '%v'.", expectedContents, m.err)
		}
	}()
	defer m.commitOrRollback(nil)
	panic("Runtime error")
}

func TestBeginTxFailure(t *testing.T) {
	m := makeTestMigrator()
	bt := BadTransactor{}
	_ = m.beginTx(bt)
	if !errors.Is(m.err, ErrBeginFailed) {
		t.Errorf("Expected error '%v'. Got '%v'.", ErrBeginFailed, m.err)
	}
}

func TestLockAndUnlockSuccess(t *testing.T) {
	withLatestDB(t, func(db *pgxpool.Pool) {
		m := makeTestMigrator()
		m.lock(db)
		if m.err != nil {
			t.Error(m.err)
		}
		m.unlock(db)
		if m.err != nil {
			t.Error(m.err)
		}
	})
}

func TestLockFailure(t *testing.T) {
	bq := BadQueryer{}
	m := makeTestMigrator()
	m.lock(bq)
	expectedContents := "FAIL: SELECT pg_advisory_lock"
	if m.err == nil || !strings.Contains(m.err.Error(), expectedContents) {
		t.Errorf("Expected error msg with '%s'. Got '%s'", expectedContents, m.err)
	}

	m.err = ErrPriorFailure
	m.lock(bq)
	if m.err != ErrPriorFailure {
		t.Errorf("Expected error %v. Got %v", ErrPriorFailure, m.err)
	}
}

func TestUnlockFailure(t *testing.T) {
	bq := BadQueryer{}
	m := makeTestMigrator()
	m.unlock(bq)
	expectedContents := "FAIL: SELECT pg_advisory_unlock"
	if m.err == nil || !strings.Contains(m.err.Error(), expectedContents) {
		t.Errorf("Expected error msg with '%s'. Got '%v'", expectedContents, m.err)
	}

	m.err = ErrPriorFailure
	m.unlock(bq)
	if m.err != ErrPriorFailure {
		t.Errorf("Expected error %v. Got %v.", ErrPriorFailure, m.err)
	}
}

func TestCreateMigrationsTable(t *testing.T) {
	withEachDB(t, func(db *pgxpool.Pool) {
		migrator := makeTestMigrator()
		migrator.createMigrationsTable(db)
		if migrator.err != nil {
			t.Errorf("Error occurred when creating migrations table: %s", migrator.err)
		}

		// Test that we can re-run it safely
		migrator.createMigrationsTable(db)
		if migrator.err != nil {
			t.Errorf("Calling createMigrationsTable a second time failed: %s", migrator.err)
		}
	})
}
func TestCreateMigrationsTableFailure(t *testing.T) {
	m := makeTestMigrator()
	bq := BadQueryer{}
	m.err = ErrPriorFailure
	m.createMigrationsTable(bq)
	if m.err != ErrPriorFailure {
		t.Errorf("Expected error %v. Got %v.", ErrPriorFailure, m.err)
	}
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

func TestRunFailure(t *testing.T) {
	bq := BadQueryer{}
	m := makeTestMigrator()
	m.run(bq, makeValidMigrations())
	expectedContents := "FAIL: SELECT id, checksum"
	if m.err == nil || !strings.Contains(m.err.Error(), expectedContents) {
		t.Errorf("Expected error msg with '%s'. Got '%v'.", expectedContents, m.err)
	}

	m.err = ErrPriorFailure
	m.run(bq, makeValidMigrations())
	if m.err != ErrPriorFailure {
		t.Errorf("Expected error %v. Got %v.", ErrPriorFailure, m.err)
	}
}

type StrLog string

func (nl *StrLog) Print(msgs ...interface{}) {
	var sb strings.Builder
	for _, msg := range msgs {
		sb.WriteString(fmt.Sprintf("%s", msg))
	}
	result := StrLog(sb.String())
	*nl = result
}

func TestSimpleLogger(t *testing.T) {
	var str StrLog
	m := NewMigrator(WithLogger(&str))
	m.log("Test message")
	if str != "Test message" {
		t.Errorf("Expected logger to print 'Test message'. Got '%s'", str)
	}
}

func makeTestMigrator() Migrator {
	tableName := time.Now().Format(time.RFC3339Nano)
	return NewMigrator(WithTableName(tableName))
}

func makeValidMigrations() []*Migration {
	return []*Migration{
		{
			ID:     "2021-01-01 001",
			Script: "CREATE TABLE first_table (created_at TIMESTAMP WITH TIME ZONE NOT NULL)",
		},
		{
			ID: "2021-01-01 002",
			Script: `CREATE TABLE data_table (
				id INTEGER GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
				created_at TIMESTAMP WITH TIME ZONE NOT NULL
			)`,
		},
		{
			ID:     "2021-01-01 003",
			Script: `INSERT INTO data_table (created_at) VALUES (NOW())`,
		},
	}
}
