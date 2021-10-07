package pgxschema

import (
	"fmt"
	"time"
)

// AppliedMigration represents a successfully-executed migration. It embeds
// Migration, and adds fields for execution results. This type is what
// records persisted in the schema_migrations table align with.
type AppliedMigration struct {
	Migration

	// Checksum is the MD5 hash of the Script for this migration
	Checksum string

	// ExecutionTimeInMillis is populated after the migration is run, indicating
	// how much time elapsed while the Script was executing.
	ExecutionTimeInMillis int

	// AppliedAt is the time at which this particular migration's Script began
	// executing (not when it completed executing).
	AppliedAt time.Time
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
