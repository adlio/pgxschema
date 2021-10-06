package pgxschema

import (
	"fmt"
)

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
