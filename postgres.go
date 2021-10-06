package pgxschema

import (
	"fmt"
	"hash/crc32"
)

const postgresAdvisoryLockSalt uint32 = 542384964

// LockSQL generates the global lock SQL statement
func LockSQL(tableName string) string {
	lockID := advisoryLockID(tableName)
	return fmt.Sprintf(`SELECT pg_advisory_lock(%s)`, lockID)
}

// UnlockSQL generates the global unlock SQL statement
func UnlockSQL(tableName string) string {
	lockID := advisoryLockID(tableName)
	return fmt.Sprintf(`SELECT pg_advisory_unlock(%s)`, lockID)
}

// CreateSQL takes the name of the migration tracking table and
// returns the SQL statement needed to create it
func CreateSQL(tableName string) string {
	return fmt.Sprintf(`
				CREATE TABLE IF NOT EXISTS %s (
					id VARCHAR(255) NOT NULL,
					checksum VARCHAR(32) NOT NULL DEFAULT '',
					execution_time_in_millis INTEGER NOT NULL DEFAULT 0,
					applied_at TIMESTAMP WITH TIME ZONE NOT NULL
				)
			`, tableName)
}

// InsertSQL takes the name of the migration tracking table and
// returns the SQL statement needed to insert a migration into it
func InsertSQL(tableName string) string {
	return fmt.Sprintf(`
				INSERT INTO %s
				( id, checksum, execution_time_in_millis, applied_at )
				VALUES
				( $1, $2, $3, $4 )
				`,
		tableName,
	)
}

// SelectSQL takes the name of the migration tracking table and
// returns trhe SQL statement to retrieve all records from it
//
func SelectSQL(tableName string) string {
	return fmt.Sprintf(`
		SELECT id, checksum, execution_time_in_millis, applied_at
		FROM %s
		ORDER BY id ASC
	`, tableName)
}

// advisoryLockID generates a table-specific lock name to use
func advisoryLockID(tableName string) string {
	sum := crc32.ChecksumIEEE([]byte(tableName))
	sum = sum * postgresAdvisoryLockSalt
	return fmt.Sprint(sum)
}
