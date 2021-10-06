package pgxschema

import (
	"crypto/md5" // #nosec MD5 only being used to fingerprint script contents, not for encryption
	"fmt"
	"sort"
	"time"
)

// Migration is a yet-to-be-run change to the schema. This is the type which
// is provided to Migrator.Apply to request a schema change.
type Migration struct {
	ID     string
	Script string
}

// MD5 computes the MD5 hash of the Script for this migration so that it
// can be uniquely identified later.
func (m *Migration) MD5() string {
	return fmt.Sprintf("%x", md5.Sum([]byte(m.Script))) // #nosec not using MD5 cryptographically
}

// AppliedMigration represents a successfully-executed migration. It embeds
// Migration, and adds fields for execution results. This type is what
// records persisted in the schema_migrations table align with.
type AppliedMigration struct {
	Migration
	Checksum              string
	ExecutionTimeInMillis int
	AppliedAt             time.Time
}

// SortMigrations sorts a slice of migrations by their IDs
func SortMigrations(migrations []*Migration) {
	// Adjust execution order so that we apply by ID
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].ID < migrations[j].ID
	})
}

// GetAppliedMigrations retrieves all already-applied migrations in a map keyed
// by the migration IDs
//
func (m Migrator) GetAppliedMigrations(db Queryer) (applied map[string]*AppliedMigration, err error) {
	applied = make(map[string]*AppliedMigration)
	migrations := make([]*AppliedMigration, 0)

	tn := QuotedTableName(m.schemaName, m.tableName)
	query := fmt.Sprintf(`
		SELECT id, checksum, execution_time_in_millis, applied_at
		FROM %s
		ORDER BY id ASC
	`, tn)

	rows, err := db.Query(m.ctx, query)
	if err != nil {
		return applied, err
	}
	defer rows.Close()

	for rows.Next() {
		migration := AppliedMigration{}
		err = rows.Scan(&migration.ID, &migration.Checksum, &migration.ExecutionTimeInMillis, &migration.AppliedAt)
		migrations = append(migrations, &migration)
	}
	for _, migration := range migrations {
		applied[migration.ID] = migration
	}
	return applied, err
}
