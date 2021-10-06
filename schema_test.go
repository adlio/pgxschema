package pgxschema

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	// Postgres database driver
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"

	"github.com/ory/dockertest"
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

var ErrBeginFailed = fmt.Errorf("Begin Failed")

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

type ConnInfo struct {
	DockerRepo string
	DockerTag  string
	DSN        string
	Resource   *dockertest.Resource
}

var DBConns = map[string]*ConnInfo{
	"postgres11": {
		DockerRepo: "postgres",
		DockerTag:  "11",
	},
}

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

		// Set the container to expire in a minute to avoid orphaned contianers
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

func TestFailedMigration(t *testing.T) {
	db := connectDB(t, "postgres11")
	migrator := makeTestMigrator()
	migrations := []*Migration{
		{
			ID:     "2019-01-01 Bad Migration",
			Script: "CREATE TIBBLE bad_table_name (id INTEGER NOT NULL PRIMARY KEY)",
		},
	}
	err := migrator.Apply(db, migrations)
	if err == nil || !strings.Contains(err.Error(), "TIBBLE") {
		t.Errorf("Expected explanatory error from failed migration. Got %v", err)
	}
	quotedTableName := QuotedTableName(migrator.schemaName, migrator.tableName)
	rows, err := db.Query(context.Background(), "SELECT * FROM "+quotedTableName)
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Error("Expected the schema table to not exist")
	}
	if rows.Next() {
		t.Error("Record was inserted in tracking table even though the migration failed")
	}
	rows.Close()
}

func TestMigrationsAppliedLexicalOrderByID(t *testing.T) {
	db := connectDB(t, "postgres11")
	tableName := "lexical_order_migrations"
	migrator := NewMigrator(WithTableName(tableName))
	outOfOrderMigrations := []*Migration{
		{
			ID:     "2019-01-01 999 Should Run Last",
			Script: "CREATE TABLE last_table (id INTEGER NOT NULL);",
		},
		{
			ID:     "2019-01-01 001 Should Run First",
			Script: "CREATE TABLE first_table (id INTEGER NOT NULL);",
		},
	}
	err := migrator.Apply(db, outOfOrderMigrations)
	if err != nil {
		t.Error(err)
	}

	applied, err := migrator.GetAppliedMigrations(db)
	if err != nil {
		t.Error(err)
	}
	if len(applied) != 2 {
		t.Errorf("Expected exactly 2 applied migrations. Got %d", len(applied))
	}
	firstMigration := applied["2019-01-01 001 Should Run First"]
	if firstMigration == nil {
		t.Error("Missing first migration")
	} else {
		if firstMigration.Checksum == "" {
			t.Error("Expected checksum to get populated when migration ran")
		}

		secondMigration := applied["2019-01-01 999 Should Run Last"]
		if secondMigration == nil {
			t.Error("Missing second migration")
		} else {
			if secondMigration.Checksum == "" {
				t.Error("Expected checksum to get populated when migration ran")
			}

			if firstMigration.AppliedAt.After(secondMigration.AppliedAt) {
				t.Errorf("Expected migrations to run in lexical order, but first migration ran at %s and second one ran at %s", firstMigration.AppliedAt, secondMigration.AppliedAt)
			}
		}

	}
}

func TestSimultaneousMigrations(t *testing.T) {
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
			db := connectDB(t, "postgres11")
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
	db := connectDB(t, "postgres11")
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
