package pgxschema

import (
	"testing"
)

func TestPostgres11MultiStatementMigrations(t *testing.T) {
	db := connectDB(t, "postgres11")
	tableName := "musicdatabase_migrations"
	migrator := NewMigrator(WithTableName(tableName))

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

	secondMigratorWithPublicSchema := NewMigrator(WithTableName("public", tableName))
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
	err = secondMigratorWithPublicSchema.Apply(db, migrationSet2)
	if err != nil {
		t.Error(err)
	}
}
