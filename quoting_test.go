package pgxschema

import "testing"

func TestQuotedTableName(t *testing.T) {
	type qtnTest struct {
		schema, table string
		expected      string
	}
	tests := []qtnTest{
		{"public", "users", `"public"."users"`},
		{"schema.with.dot", "table.with.dot", `"schema.with.dot"."table.with.dot"`},
		{`public"`, `"; DROP TABLE users`, `"public"."DROPTABLEusers"`},
	}
	for _, test := range tests {
		actual := QuotedTableName(test.schema, test.table)
		if actual != test.expected {
			t.Errorf("Expected %s, got %s", test.expected, actual)
		}
	}
}

func TestQuotedIdent(t *testing.T) {
	table := map[string]string{
		"MY_TABLE":           `"MY_TABLE"`,
		"users_roles":        `"users_roles"`,
		"table.with.dot":     `"table.with.dot"`,
		`table"with"quotes"`: `"tablewithquotes"`,
	}
	for ident, expected := range table {
		actual := QuotedIdent(ident)
		if expected != actual {
			t.Errorf("Expected %s, got %s", expected, actual)
		}
	}
}
