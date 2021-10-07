package pgxschema

import "testing"

func TestMD5(t *testing.T) {
	m := Migration{
		Script: `CREATE TABLE my_table (id INTEGER GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY)`,
	}
	hash := m.MD5()
	expected := "753022e5f4a2fb8f5f8eb849e5057e05"
	if hash != expected {
		t.Errorf("Expected hash '%s', got '%s'", expected, m.MD5())
	}
}

