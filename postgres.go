package pgxschema

import (
	"fmt"
)

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
