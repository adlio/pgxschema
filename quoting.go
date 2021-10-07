package pgxschema

import (
	"hash/crc32"
	"strings"
	"unicode"
)

const postgresAdvisoryLockSalt = 542384964

// QuotedTableName returns the string value of the name of the migration
// tracking table after it has been quoted for Postgres
//
func QuotedTableName(schemaName, tableName string) string {
	if schemaName == "" {
		return QuotedIdent(tableName)
	}
	return QuotedIdent(schemaName) + "." + QuotedIdent(tableName)
}

// QuotedIdent transforms the provided string into a valid, quoted Postgres
// identifier. This
func QuotedIdent(ident string) string {
	if ident == "" {
		return ""
	}

	var sb strings.Builder
	sb.WriteRune('"')
	for _, r := range ident {
		switch {
		case unicode.IsSpace(r):
			// Skip spaces
			continue
		case r == '"':
			// Escape double-quotes with repeated double-quotes
			sb.WriteString(`""`)
		case r == ';':
			// Ignore the command termination character
			continue
		default:
			sb.WriteRune(r)
		}
	}
	sb.WriteRune('"')
	return sb.String()
}

// LockIdentifierForTable computes a hash of the migrations table's name which
// can be used as a unique name for the Postgres advisory lock
//
func LockIdentifierForTable(tableName string) int64 {
	sum := crc32.ChecksumIEEE([]byte(tableName))
	return int64(sum) * postgresAdvisoryLockSalt
}
