package pgxschema

import (
	"strings"
	"unicode"
)

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
