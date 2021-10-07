# PGX Schema - Embedded Database Migration Library for Go (PGX Driver Version)

[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white&style=for-the-badge)](https://pkg.go.dev/adlio/pgxschema)
[![CircleCI Build Status](https://img.shields.io/circleci/build/gh/adlio/pgxschema/main?style=for-the-badge)](https://circleci.com/gh/adlio/pgxschema/tree/main)
[![Go Report Card](https://goreportcard.com/badge/github.com/adlio/pgxschema?style=for-the-badge)](https://goreportcard.com/report/github.com/adlio/pgxschema)
[![Code Coverage](https://img.shields.io/codecov/c/github/adlio/pgxschema?style=for-the-badge)](https://codecov.io/gh/adlio/pgxschema)

An opinionated, embedded library for tracking and applying modifications
to the PostgreSQL schema from inside a Go application using the
[jackc/pgx](https://github.com/jackc/pgx) driver. If you use a `database/sql`
driver instead, please see the related
[adlio/schema](https://github.com/adlio/schema) package.

Tools like
[goose](https://github.com/pressly/goose) and
[golang-migrate](https://github.com/golang-migrate/migrate) treat database
migrations and the application as separate entities. This creates a deploy-time
burden; somebody has to automate a process to get the migrations run at the
right time before the new code is activated in an environment.

This package takes a different approach. By requiring migrations be "baked in"
to the Go binary, the migrations can be run at application startup regardless
of the environment.

Every approach has tradeoffs. If your project involves a DBA applying
schema changes manually, this library is definitely not for you. On the other hand
if you're distributing your application to end-users via a standalone Go binary,
this approach might be perfect.

# Usage Instructions

Create a `pgxschema.Migrator` in your bootstrap/config/database connection code,
then call its `Apply()` method with your database connection and a slice of
`*pgxschema.Migration` structs. Like so:

    db, err := pgxpool.Connect() // or pgx.Connect()

    migrator := pgxpgxschema.NewMigrator()
    migrator.Apply(db, []*pgxschema.Migration{
      &pgxschema.Migration{
        ID: "2019-09-24 Create Albums",
        Script: `
        CREATE TABLE albums (
          id SERIAL PRIMARY KEY,
          title CHARACTER VARYING (255) NOT NULL
        )
        `
      }
    })

The `.Apply()` function figures out which of the supplied Migrations have not
yet been executed in the database (based on the ID), and executes the `Script`
for each in **alphabetical order by ID**. This procedure means its OK to call
`.Apply()` on the same Migrator with a different set of Migrations each time

# Customization Options

The `NewMigrator()` function accepts option arguments to customize its behavior.

## WithTableName

By default, the tracking table will be placed in the schema from the
`search_path`, and it will be named `schema_migrations`. This behavior can
be changed by supplying a `WithTableName()` option to the `NewMigrator()`.

```go
m := pgxschema.NewMigrator(pgxschema.WithTableName("my_migrations"))
```

If you need to customize both the schema and the table name, provide two
arguments:

```go
m := pgxschema.NewMigrator(pgxschema.WithTableName("my_schema", "my_migrations"))
```

**NOTE**: Providing a schema like so does not influence the behavior SQL run
inside your migrations. If a migration needs to `CREATE TABLE` in a specific
schema, that will need to be specified inside the migration itself.

It is theoretically possible to create multiple Migrators and to use mutliple
migration tracking tables within the same application and database.

# Concurrent Execution Support

The `pgxschema` package utilizes
[PostgreSQL Advisory Locks](https://www.postgresql.org/docs/13/explicit-locking.html#ADVISORY-LOCKS)
to ensure that only one process can run migrations at a time.

This allows multiple **processes** (not just goroutines) to run `Apply` on
identically configured migrators simultaneously. The first-arriving process
will **win** and perform all needed migrations on the database. All other
processses will wait until the lock is released, after which they'll each
obtain the lock and run `Apply()` which should be a no-op based on the
first-arriving process' successful completion.

# Migration Ordering

Migrations **are not** executed in the order they are specified in the slice.
They will be re-sorted alphabetically by their IDs before executing them.

# Rules for Writing Migrations

1.  **Never, ever change** the `ID` of a Migration which has already
    been executed on your database. Doing so will cause the system to recognize
    this as a new migration which needs to be applied again.
2.  Use a consistent, but descriptive format for migration IDs. A recommended
    format is to use the timestamp as a prefix followed by a decriptive phrase:

         ID: "2019-01-01T13:45 Creates Users"
         ID: "2019-01-10T10:33 Creates Artists"

    Do not use simple sequentialnumbers like `ID: "1"` with a distributed team
    unless you have a reliable process for developers to "claim" the next ID.

# Inspecting the State of Applied Migrations

Call `migrator.GetAppliedMigrations(db)` to get info about migrations which
have been successfully applied.

# TODO List

- [x] Port `adlio/schema` to a `jackc/pgx`-friendly version
- [x] Alter transaction handling to be more PostgreSQL-specific
- [x] 100% test coverage, including running against multiple PostgreSQL versions
- [ ] Support for creating []\*Migration from a Go 1.16 `embed.FS`
- [ ] Documentations for using Go 1.16 // go:embed to populate Script variables
- [ ] Options for alternative failure behavior when `pg_advisory_lock()` takes too long.
      The current behavior should allow for early failure by providing a context with a
      timeout to `WithContext()`, but this hasn't been tested.

# Contributions

... are welcome. Please include tests with your contribution. We've integrated
[dockertest](https://github.com/ory/dockertest) to automate the process of
creating clean test databases.

## Package Opinions

There are many other schema migration tools. This one exists because of a
particular set of opinions:

1. Database credentials are runtime configuration details, but database
   schema is a **build-time applicaton dependency**, which means it should be
   "compiled in" to the build, and should not rely on external tools.
2. Using an external command-line tool for schema migrations needlessly
   complicates testing and deployment.
3. Sequentially-numbered integer migration IDs will create too many unnecessary
   schema collisions on a distributed, asynchronously-communicating team.
4. SQL is the best language to use to specify changes to SQL schemas.
5. "Down" migrations add needless complication, aren't often used, and are
   tedious to properly test when they are used. In the unlikely event you need
   to migrate backwards, it's possible to write the "rollback" migration as
   a separate "up" migration.
6. Deep dependency chains should be avoided, especially in a compiled
   binary. We don't want to import an ORM into our binaries just to get SQL
   the features of this package. The `pgxschema` package imports only
   [standard library packages](https://godoc.org/github.com/adlio/pgxschema?imports)
   and the `jackc/pgx` driver code.
   (**NOTE** \*We do import `ory/dockertest` to automate testing on various
   PostgreSQL versions via docker).
7. Finally... storing raw SQL as strings inside `.go` files is an acceptable
   trade-off for the above.

## Testing Procedure

Testing requires a Docker daemon running on your test machine to spin-up
temporary PostgreSQL database servers to test against.

```bash
go test -v -cover
```

# Version History

## 0.0.1 (October 7, 2021)

- First port from `adlio/schema`

# License

Copyright (c) 2021 Aaron Longwell, released under the MIT License.
