package pgxschema

import (
	"context"
	"log"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/ory/dockertest"
)

// TestMain replaces the normal test runner for this package. It connects to
// Docker running on the local machine and launches testing database
// containers to which we then connect and store the connection in a package
// global variable
//
func TestMain(m *testing.M) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("Can't run pgxschema tests. Docker is not running: %s", err)
	}

	var wg sync.WaitGroup
	for name := range TestDBs {
		testDB := TestDBs[name]
		wg.Add(1)
		go func() {
			testDB.Init(pool)
			wg.Done()
		}()
	}
	wg.Wait()

	code := m.Run()

	// Purge all the containers we created
	// You can't defer this because os.Exit doesn't execute defers
	for _, info := range TestDBs {
		info.Cleanup(pool)
	}

	os.Exit(code)
}

// withLatestDB runs the provided function with a connection to the most recent
// version of PostgreSQL
func withLatestDB(t *testing.T, f func(db *pgxpool.Pool)) {
	db := connectDB(t, "postgres:latest")
	f(db)
}

// withEachDB runs the provided function with a connection to all PostgreSQL
// versions defined in the DBConns constant
func withEachDB(t *testing.T, f func(db *pgxpool.Pool)) {
	t.Helper()
	for dbName := range TestDBs {
		t.Run(dbName, func(t *testing.T) {
			db := connectDB(t, dbName)
			f(db)
		})
	}
}

// connectDB opens a connection to the PostgreSQL docker container with the
// provided key name.
func connectDB(t *testing.T, name string) *pgxpool.Pool {
	t.Helper()
	info, exists := TestDBs[name]
	if !exists {
		t.Errorf("Database '%s' doesn't exist.", name)
	}
	db, err := pgxpool.Connect(context.Background(), info.DSN())
	if err != nil {
		t.Error(err)
	}
	return db
}
