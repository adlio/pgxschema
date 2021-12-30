package pgxschema

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)

// TestCreateMigrationsTable ensures that each test datbase can
// successfully create the schema_migrations table.
func TestCreateMigrationsTable(t *testing.T) {
	withEachDB(t, func(db *pgxpool.Pool) {
		migrator := makeTestMigrator()
		err := migrator.createMigrationsTable(db)
		if err != nil {
			t.Errorf("Error occurred when creating migrations table: %s", err)
		}

		// Test that we can re-run it safely
		err = migrator.createMigrationsTable(db)
		if err != nil {
			t.Errorf("Calling createMigrationsTable a second time failed: %s", err)
		}
	})
}

// TestLockAndUnlock tests the Lock and Unlock mechanisms of each
// test database in isolation from any migrations actually being run.
func TestLockAndUnlock(t *testing.T) {
	withEachDB(t, func(db *pgxpool.Pool) {
		m := makeTestMigrator()
		err := m.lock(db)
		if err != nil {
			t.Error(err)
		}
		err = m.unlock(db)
		if err != nil {
			t.Error(err)
		}
	})
}

// TestApplyInLexicalOrder ensures that each test database runs migrations in
// lexical order rather than the order they were provided in the slice. This is
// also the primary test to assert that the data in the tracking table is
// all correct.
func TestApplyInLexicalOrder(t *testing.T) {
	withEachDB(t, func(db *pgxpool.Pool) {

		start := time.Now().Truncate(time.Second) // MySQL has only second accuracy, so we need start/end to span 1 second

		tableName := "lexical_order_migrations"
		migrator := NewMigrator(WithTableName(tableName))
		err := migrator.Apply(db, unorderedMigrations())
		if err != nil {
			t.Error(err)
		}

		end := time.Now().Add(time.Second).Truncate(time.Second) // MySQL has only second accuracy, so we need start/end to span 1 second

		applied, err := migrator.GetAppliedMigrations(db)
		if err != nil {
			t.Error(err)
		}
		if len(applied) != 3 {
			t.Errorf("Expected exactly 2 applied migrations. Got %d", len(applied))
		}

		firstMigration := applied["2021-01-01 001"]
		if firstMigration == nil {
			t.Fatal("Missing first migration")
		}
		if firstMigration.Checksum == "" {
			t.Error("Expected non-blank Checksum value after successful migration")
		}
		if firstMigration.ExecutionTimeInMillis < 1 {
			t.Errorf("Expected ExecutionTimeInMillis of %s to be tracked. Got %d", firstMigration.ID, firstMigration.ExecutionTimeInMillis)
		}
		// Put value in consistent timezone to aid error message readability
		appliedAt := firstMigration.AppliedAt.Round(time.Second)
		if appliedAt.IsZero() || appliedAt.Before(start) || appliedAt.After(end) {
			t.Errorf("Expected AppliedAt between %s and %s, got %s", start, end, appliedAt)
		}
		assertZonesMatch(t, start, appliedAt)

		secondMigration := applied["2021-01-01 002"]
		if secondMigration == nil {
			t.Fatal("Missing second migration")
		} else if secondMigration.Checksum == "" {
			t.Fatal("Expected checksum to get populated when migration ran")
		}

		if firstMigration.AppliedAt.After(secondMigration.AppliedAt) {
			t.Errorf("Expected migrations to run in lexical order, but first migration ran at %s and second one ran at %s", firstMigration.AppliedAt, secondMigration.AppliedAt)
		}
	})
}

// TestFailedMigration ensures that a migration with a syntax error triggers
// an expected error when Apply() is run. This test is run on every test database
func TestFailedMigration(t *testing.T) {
	withEachDB(t, func(db *pgxpool.Pool) {
		tableName := time.Now().Format(time.RFC3339Nano)
		migrator := NewMigrator(WithTableName(tableName))
		migrations := []*Migration{
			{
				ID:     "2019-01-01 Bad Migration",
				Script: "CREATE TIBBLE bad_table_name (id INTEGER NOT NULL PRIMARY KEY)",
			},
		}
		err := migrator.Apply(db, migrations)
		expectErrorContains(t, err, "TIBBLE")

		query := "SELECT * FROM " + migrator.QuotedTableName()
		rows, _ := db.Query(context.Background(), query)
		defer rows.Close()

		// We expect either an error (because the transaction was rolled back
		// and the table no longer exists)... or  a query with no results
		if rows != nil {
			if rows.Next() {
				t.Error("Record was inserted in tracking table even though the migration failed")
			}
		}
	})
}

// TestSimultaneousApply creates multiple Migrators and multiple distinct
// connections to each test database and attempts to call .Apply() on them all
// concurrently. The migrations include an INSERT statement, which allows us
// to count to ensure that each unique migration was only run once.
//
func TestSimultaneousApply(t *testing.T) {
	concurrency := 4
	dataTable := fmt.Sprintf("data%d", rand.Int()) // #nosec don't need a strong RNG here
	migrationsTable := fmt.Sprintf("Migrations %s", time.Now().Format(time.RFC3339Nano))
	sharedMigrations := []*Migration{
		{
			ID:     "2020-05-01 Sleep",
			Script: "SELECT pg_sleep(1)",
		},
		{
			ID: "2020-05-02 Create Data Table",
			Script: fmt.Sprintf(`CREATE TABLE %s (
				id INTEGER GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
				created_at TIMESTAMP WITH TIME ZONE NOT NULL
			)`, dataTable),
		},
		{
			ID:     "2020-05-03 Add Initial Record",
			Script: fmt.Sprintf(`INSERT INTO %s (created_at) VALUES (NOW())`, dataTable),
		},
	}

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(i int) {
			db := connectDB(t, "postgres:latest")
			migrator := NewMigrator(WithTableName(migrationsTable))
			err := migrator.Apply(db, sharedMigrations)
			if err != nil {
				t.Error(err)
			}
			_, err = db.Exec(context.Background(), fmt.Sprintf("INSERT INTO %s (created_at) VALUES (NOW())", dataTable))
			if err != nil {
				t.Error(err)
			}
			wg.Done()
		}(i)
	}
	wg.Wait()

	// We expect concurrency + 1 rows in the data table
	// (1 from the migration, and one each for the
	// goroutines which ran Apply and then did an
	// insert afterwards)
	db := connectDB(t, "postgres:latest")
	count := 0
	row := db.QueryRow(context.Background(), fmt.Sprintf("SELECT COUNT(*) FROM %s", dataTable))
	err := row.Scan(&count)
	if err != nil {
		t.Error(err)
	}
	if count != concurrency+1 {
		t.Errorf("Expected to get %d rows in %s table. Instead got %d", concurrency+1, dataTable, count)
	}
}

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

// makeTestMigrator is a utility function which produces a migrator with an
// isolated environment (isolated due to a unique name for the migration
// tracking table).
func makeTestMigrator() Migrator {
	tableName := time.Now().Format(time.RFC3339Nano)
	return NewMigrator(WithTableName(tableName))
}

// assertZonesMatch accepts two Times and fails the test if their time zones
// don't match.
func assertZonesMatch(t *testing.T, expected, actual time.Time) {
	t.Helper()
	expectedName, expectedOffset := expected.Zone()
	actualName, actualOffset := actual.Zone()
	if expectedOffset != actualOffset {
		t.Errorf("Expected Zone '%s' with offset %d. Got Zone '%s' with offset %d", expectedName, expectedOffset, actualName, actualOffset)
	}
}
