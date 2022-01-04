# PGX Schema - Embedded Database Migration Library for Go (PGX Driver Version)

An embeddable library for tracking and applying modifications
to the PostgreSQL schema from inside a Go application using the
[jackc/pgx](https://github.com/jackc/pgx) driver.

[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white&style=for-the-badge)](https://pkg.go.dev/adlio/pgxschema)
[![CircleCI Build Status](https://img.shields.io/circleci/build/gh/adlio/pgxschema/main?style=for-the-badge)](https://circleci.com/gh/adlio/pgxschema/tree/main)
[![Go Report Card](https://goreportcard.com/badge/github.com/adlio/pgxschema?style=for-the-badge)](https://goreportcard.com/report/github.com/adlio/pgxschema)
[![Code Coverage](https://img.shields.io/codecov/c/github/adlio/pgxschema?style=for-the-badge)](https://codecov.io/gh/adlio/pgxschema)

**NOTE**: If you use a `database/sql` driver instead, please see the related
[adlio/schema](https://github.com/adlio/schema) package.

## Features

- Cloud-friendly design tolerates embedded use in clusters
- Supports migrations in embed.FS (requires go:embed in Go 1.16+)
- [Depends only on Go standard library and jackc/pgx](https://pkg.go.dev/github.com/adlio/pgxschema?tab=imports) (Note that all go.mod dependencies are used only in tests)
- Unidirectional migrations (no "down" migration complexity)

# Usage Instructions

Create a `pgxschema.Migrator` in your bootstrap/config/database connection code,
then call its `Apply()` method with your database connection and a slice of
`*pgxschema.Migration` structs.

The `.Apply()` function figures out which of the supplied Migrations have not
yet been executed in the database (based on the ID), and executes the `Script`
for each in **alphabetical order by ID**.

The `[]*pgxschema.Migration` can be created manually, but the package has some
utility functions to make it easier to read .sql files into structs, with the
filename as the `ID` and the contents being the `Script`.

## Using go:embed (requires Go 1.16+)

Go 1.16 added features to embed a directory of files into the binary as an
embedded filesystem (`embed.FS`).

Assuming you have a directory of SQL files called `my-migrations/` next to your
main.go file, you'll run something like this (the comments with go:embed are
relevant).

```go
//go:embed my-migrations
var MyMigrations embed.FS

func main() {
   db, err := pgxpool.Connect() // or pgx.Connect()

   migrator := pgxschema.NewMigrator()
   err = migrator.Apply(
      db,
      pgxschema.FSMigrations(MyMigrations, "my-migrations/*.sql"),
   )
}
```

The result will be a slice of `*pgxschema.Migration{}` with the file's name
(without the extension) as the `ID` property, and the entire contents of the
file as its `Script` property. The `test-migrations/saas` directory provides an
example.

## Using Inline Migration Structs

If you're running an earlier version of Go, Migration{} structs will need to be
created manually:

```go
db, err := pgxpool.Connect() // or pgx.Connect()

migrator := pgxschema.NewMigrator()
migrator.Apply(db, []*pgxschema.Migration{
   &pgxschema.Migration{
      ID: "2019-09-24 Create Albums",
      Script: `
      CREATE TABLE albums (
         id SERIAL PRIMARY KEY,
         title CHARACTER VARYING (255) NOT NULL
      )
      `,
   },
})
```

# Constructor Options

The `NewMigrator()` function accepts option arguments to customize its behavior.

## WithTableName

By default, the tracking table will be placed in the schema from the
search_path, and it will be named `schema_migrations`. This behavior can
be changed by supplying a WithTableName() option to the NewMigrator() call.

```go
m := pgxschema.NewMigrator(pgxschema.WithTableName("my_migrations"))
```

If you need to customize both the schema and the table name, provide two
arguments:

```go
m := pgxschema.NewMigrator(pgxschema.WithTableName("my_schema", "my_migrations"))
```

**NOTE**: Providing a schema like so does not influence the behavior of SQL run
inside your migrations. If a migration needs to `CREATE TABLE` in a specific
schema, that will need to be specified inside the migration itself or configured
via the `search_path` when opening a connection.

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

## Rules for Writing Migrations

1.  **Never, ever change** the `ID` (filename) or `Script` (file contents)
    of a Migration which has already been executed on your database. If you've
    made a mistake, you'll need to correct it in a subsequent migration.
2.  Use a consistent, but descriptive format for migration `ID`s/filenames.
    Consider prefixing them with today's timestamp. Examples:

          ID: "2019-01-01T13:45 Creates Users"
          ID: "2019-01-10T10:33 Creates Artists"

    Do not use simple sequential numbers like `ID: "1"` with a distributed team
    unless you have a reliable process for developers to "claim" the next ID.

# Contributions

... are welcome. Please include tests with your contribution. We've integrated
[dockertest](https://github.com/ory/dockertest) to automate the process of
creating clean test databases.

## Testing Procedure

Testing requires a Docker daemon running on your test machine to spin-up
temporary PostgreSQL database servers to test against. Ensure your contribution
keeps test coverage high and passes all existing tests.

```bash
go test -v -cover
```

## Package Opinions

There are many other schema migration tools. This one exists because of a
particular set of opinions:

1. Database credentials are runtime configuration details, but database
   schema is a **build-time applicaton dependency**, which means it should be
   "compiled in" to the build, and should not rely on external tools.
2. Using an external command-line tool for schema migrations needlessly
   complicates testing and deployment.
3. SQL is the best language to use to specify changes to SQL schemas.
4. "Down" migrations add needless complication, aren't often used, and are
   tedious to properly test when they are used. In the unlikely event you need
   to migrate backwards, it's possible to write the "rollback" migration as
   a separate "up" migration.
5. Deep dependency chains should be avoided, especially in a compiled
   binary. We don't want to import an ORM into our binaries just to get SQL
   querying support. The `pgxschema` package imports only
   [standard library packages](https://godoc.org/github.com/adlio/pgxschema?imports)
   and the `jackc/pgx` driver code.
   (**NOTE** \*We do import `ory/dockertest` to automate testing on various
   PostgreSQL versions via docker).

# Roadmap

- [x] Port `adlio/schema` to a `jackc/pgx`-friendly version
- [x] Alter transaction handling to be more PostgreSQL-specific
- [x] 100% test coverage, including running against multiple PostgreSQL versions
- [x] Support for creating []\*Migration from a Go 1.16 `embed.FS`
- [x] Documentation for using Go 1.16 // go:embed to populate Script variables
- [ ] Options for alternative failure behavior when `pg_advisory_lock()` takes too long.
      The current behavior should allow for early failure by providing a context with a
      timeout to `WithContext()`, but this hasn't been tested.
- [ ] Add a `Validate()` method to allow checking migration names for
      consistency and to detect problematic changes in the migrations list

# Version History

## 1.0.0 - Jan 4, 2022

- Add support for migrations in an embed.FS (`FSMigrations(filesystem fs.FS, glob string)`)
- Update go.mod to `go 1.17`
- Simplify Apply() routine, improve test coverage
- Security updates to upstream dependencies

## 0.0.3 - Dec 10, 2021

Security updates to upstream dependencies.

## 0.0.2 - Nov 18, 2021

Security updates to upstream dependencies.

## 0.0.1 - Oct 7, 2021

First port from `adlio/schema`.

# License

Copyright (c) 2022 Aaron Longwell, released under the MIT License.
