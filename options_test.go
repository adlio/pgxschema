package pgxschema

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
)

func TestWithTableNameOptionWithSchema(t *testing.T) {
	schema := "special"
	table := "my_migrations"
	m := NewMigrator(WithTableName(schema, table))
	if m.schemaName != schema {
		t.Errorf("Expected SchemaName to be '%s'. Got '%s' instead.", schema, m.schemaName)
	}
	if m.tableName != table {
		t.Errorf("Expected TableName to be '%s'. Got '%s' instead.", table, m.tableName)
	}
}

func TestWithTableNameOptionWithoutSchema(t *testing.T) {
	name := "terrible_migrations_table_name"
	m := NewMigrator(WithTableName(name))
	if m.schemaName != "" {
		t.Errorf("Expected SchemaName to be blank. Got '%s' instead.", m.schemaName)
	}
	if m.tableName != name {
		t.Errorf("Expected TableName to be '%s'. Got '%s' instead.", name, m.tableName)
	}
}

func TestWithTableNameOptionWithNoArgs(t *testing.T) {
	m := NewMigrator(WithTableName())
	if m.schemaName != "" {
		t.Errorf("Expected SchemaName to be blank. Got '%s' instead.", m.schemaName)
	}
	if m.tableName != DefaultTableName {
		t.Errorf("Expected TableName to be the default '%s'. Got '%s' instead.", DefaultTableName, m.tableName)
	}
}

func TestDefaultTableName(t *testing.T) {
	name := "schema_migrations"
	m := NewMigrator()
	if m.schemaName != "" {
		t.Errorf("Expected SchemaName to be blank by default. Got '%s' instead.", m.schemaName)
	}
	if m.tableName != name {
		t.Errorf("Expected TableName to be '%s' by default. Got '%s' instead.", name, m.tableName)
	}
}

type testCtxKey int

const KeyFoo testCtxKey = iota

func TestWithContextOption(t *testing.T) {
	m := Migrator{}
	if m.ctx != nil {
		t.Errorf("Expected nil ctx by default. Got '%v'.", m.ctx)
	}
	ctx := context.WithValue(context.Background(), KeyFoo, "123456")
	modifiedMigrator := WithContext(ctx)(m)
	if modifiedMigrator.ctx == nil {
		t.Errorf("Expected non-nil ctx after applying WithContext")
	}
	val := modifiedMigrator.ctx.Value(KeyFoo)
	if val.(string) != "123456" {
		t.Error("Context didn't contain expected value")
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

type StrLog string

func (nl *StrLog) Print(msgs ...interface{}) {
	var sb strings.Builder
	for _, msg := range msgs {
		sb.WriteString(fmt.Sprintf("%s", msg))
	}
	result := StrLog(sb.String())
	*nl = result
}

func TestSimpleLogger(t *testing.T) {
	var str StrLog
	m := NewMigrator(WithLogger(&str))
	m.log("Test message")
	if str != "Test message" {
		t.Errorf("Expected logger to print 'Test message'. Got '%s'", str)
	}
}
