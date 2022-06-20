package pgxschema

import (
	"context"
	"fmt"
	"log"
	"testing"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
)

type TestDB struct {
	DockerRepo string
	DockerTag  string
	Resource   *dockertest.Resource
}

func (c *TestDB) Username() string {
	return "pgxschemauser"
}

func (c *TestDB) Password() string {
	return "pgxschemasecret"
}

func (c *TestDB) DatabaseName() string {
	return "pgxschematests"
}

// Port asks Docker for the host-side port we can use to connect to the
// relevant container's database port.
func (c *TestDB) Port() string {
	return c.Resource.GetPort("5432/tcp")
}

// DSN produces the connection string which is used to connect to this test
// database instance
func (c *TestDB) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@localhost:%s/%s?sslmode=disable", c.Username(), c.Password(), c.Port(), c.DatabaseName())
}

// DockerEnvars computes the environment variables that are needed for a
// docker instance.
//
func (c *TestDB) DockerEnvars() []string {
	return []string{
		fmt.Sprintf("POSTGRES_USER=%s", c.Username()),
		fmt.Sprintf("POSTGRES_PASSWORD=%s", c.Password()),
		fmt.Sprintf("POSTGRES_DB=%s", c.DatabaseName()),
	}
}

// Init sets up a test database instance for connections. For dockertest-based
// instances, this function triggers the `docker run` call. For SQLite-based
// test instances, this creates the data file. In all cases, we verify that
// the database is connectable via a test connection.
//
func (c *TestDB) Init(pool *dockertest.Pool) {
	var err error

	// For Docker-based test databases, we send a startup signal to have Docker
	// launch a container for this test run.
	log.Printf("Starting docker container %s:%s\n", c.DockerRepo, c.DockerTag)

	// The container is started with AutoRemove: true, and a restart policy to
	// not restart
	c.Resource, err = pool.RunWithOptions(&dockertest.RunOptions{
		Repository: c.DockerRepo,
		Tag:        c.DockerTag,
		Env:        c.DockerEnvars(),
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{
			Name: "no",
		}
	})

	if err != nil {
		log.Fatalf("Could not start container %s:%s: %s", c.DockerRepo, c.DockerTag, err)
	}

	// Even if everything goes OK, kill off the container after n seconds
	_ = c.Resource.Expire(60)

	// Wait for the database to come online
	err = pool.Retry(func() error {
		testConn, err := pgx.Connect(context.Background(), c.DSN())
		if err != nil {
			return err
		}
		defer testConn.Close(context.Background())
		return testConn.Ping(context.Background())
	})
	if err != nil {
		log.Fatalf("Could not connect to %s: %s", c.DSN(), err)
	} else {
		log.Printf("Successfully connected to %s", c.DSN())
	}
}

// Connect creates an additional *pgxpool.Pool connection for a particular
// test database.
//
func (c *TestDB) Connect(t *testing.T) *pgxpool.Pool {
	db, err := pgxpool.Connect(context.Background(), c.DSN())
	if err != nil {
		t.Error(err)
	}
	return db
}

// Cleanup should be called after all tests with a database instance are
// complete.
//
func (c *TestDB) Cleanup(pool *dockertest.Pool) {
	if c.Resource != nil {
		err := pool.Purge(c.Resource)
		if err != nil {
			log.Fatalf("Could not cleanup %s: %s", c.DSN(), err)
		}
	}
}
