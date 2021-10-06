package pgxschema

import (
	"fmt"
	"hash/crc32"
	"strings"
	"unicode"
)

const postgresAdvisoryLockSalt uint32 = 542384964

// QuotedTableName returns the string value of the name of the migration
// tracking table after it has been quoted for Postgres
//
func QuotedTableName(schemaName, tableName string) string {
	if schemaName == "" {
		return QuotedIdent(tableName)
	}
	return QuotedIdent(schemaName) + "." + QuotedIdent(tableName)
}

// QuotedIdent wraps the supplied string in the Postgres identifier
// quote character
func QuotedIdent(ident string) string {
	return `"` + strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		if r == '"' {
			return -1
		}
		if r == ';' {
			return -1
		}
		return r
	}, ident) + `"`
}

// LockIdentifierForTable computes a hash of the migrations table's name which
// can be used as a unique name for the Postgres advisory lock
//
func LockIdentifierForTable(tableName string) string {
	sum := crc32.ChecksumIEEE([]byte(tableName))
	sum = sum * postgresAdvisoryLockSalt
	return fmt.Sprint(sum)
}
