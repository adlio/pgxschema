package pgxschema

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/jackc/pgx/v4"
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
		log.Fatalf("Can't run schema tests. Docker is not running: %s", err)
	}

	for _, info := range DBConns {
		// Provision the container
		info.Resource, err = pool.Run(info.DockerRepo, info.DockerTag, []string{
			"POSTGRES_USER=postgres",
			"POSTGRES_PASSWORD=secret",
			"POSTGRES_DB=schematests",
		})
		if err != nil {
			log.Fatalf("Could not start container %s:%s: %s", info.DockerRepo, info.DockerTag, err)
		}

		// Set the container to expire in a minute to avoid orphaned containers
		// hanging around
		err = info.Resource.Expire(60)
		if err != nil {
			log.Fatalf("Could not set expiration time for docker test containers: %s", err)
		}

		// Save the DSN to make new connections later
		info.DSN = fmt.Sprintf("postgres://postgres:secret@localhost:%s/schematests?sslmode=disable", info.Resource.GetPort("5432/tcp"))

		// Wait for the database to come online
		if err = pool.Retry(func() error {
			testConn, err := pgx.Connect(context.Background(), info.DSN)
			if err != nil {
				return err
			}
			defer testConn.Close(context.Background())
			return testConn.Ping(context.Background())
		}); err != nil {
			log.Fatalf("Could not connect to container: %s", err)
			return
		}
	}

	code := m.Run()

	// Purge all the containers we created
	// You can't defer this because os.Exit doesn't execute defers
	for _, info := range DBConns {
		if err := pool.Purge(info.Resource); err != nil {
			log.Fatalf("Could not purge	resource: %s", err)
		}
	}

	os.Exit(code)
}

// withLatestDB runs the provided function with a connection to the most recent
// version of PostgreSQL
func withLatestDB(t *testing.T, f func(db *pgxpool.Pool)) {
	db := connectDB(t, "postgres13")
	f(db)
}

// withEachDB runs the provided function with a connection to all PostgreSQL
// versions defined in the DBConns constant
func withEachDB(t *testing.T, f func(db *pgxpool.Pool)) {
	for dbName := range DBConns {
		t.Run(dbName, func(t *testing.T) {
			db := connectDB(t, dbName)
			f(db)
		})
	}
}

// connectDB opens a connection to the PostgreSQL docker container with the
// provided key name.
func connectDB(t *testing.T, name string) *pgxpool.Pool {
	info, exists := DBConns[name]
	if !exists {
		t.Errorf("Database '%s' doesn't exist.", name)
	}
	db, err := pgxpool.Connect(context.Background(), info.DSN)
	if err != nil {
		t.Error(err)
	}
	return db
}
