package pgxschema

import (
	"log"
	"os"
	"testing"
)

func TestWithTableNameOptionWithSchema(t *testing.T) {
	schema := "special"
	table := "my_migrations"
	m := NewMigrator(WithTableName(schema, table))
	if m.SchemaName != schema {
		t.Errorf("Expected SchemaName to be '%s'. Got '%s' instead.", schema, m.SchemaName)
	}
	if m.TableName != table {
		t.Errorf("Expected TableName to be '%s'. Got '%s' instead.", table, m.TableName)
	}
}
func TestWithTableNameOptionWithoutSchema(t *testing.T) {
	name := "terrible_migrations_table_name"
	m := NewMigrator(WithTableName(name))
	if m.SchemaName != "" {
		t.Errorf("Expected SchemaName to be blank. Got '%s' instead.", m.SchemaName)
	}
	if m.TableName != name {
		t.Errorf("Expected TableName to be '%s'. Got '%s' instead.", name, m.TableName)
	}
}

func TestDefaultTableName(t *testing.T) {
	name := "schema_migrations"
	m := NewMigrator()
	if m.SchemaName != "" {
		t.Errorf("Expected SchemaName to be blank by default. Got '%s' instead.", m.SchemaName)
	}
	if m.TableName != name {
		t.Errorf("Expected TableName to be '%s' by default. Got '%s' instead.", name, m.TableName)
	}
}

func TestWithLoggerOption(t *testing.T) {
	m := Migrator{}
	if m.Logger != nil {
		t.Errorf("Expected nil Logger by default. Got '%v'", m.Logger)
	}
	modifiedMigrator := WithLogger(log.New(os.Stdout, "schema: ", log.Ldate|log.Ltime))(m)
	if modifiedMigrator.Logger == nil {
		t.Errorf("Expected logger to have been added")
	}
}
