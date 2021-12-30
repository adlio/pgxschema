package pgxschema

import (
	"regexp"
	"testing"
)

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

func TestSortMigrations(t *testing.T) {
	migrations := makeValidUnorderedMigrations()
	expectedFirst := "2021-01-01 001"
	expectedSecond := "2021-01-01 002"
	expectedThird := "2021-01-01 003"

	// First we test that the unordered set is unordered in the way
	// we expect. The valid migrations come out in 2,1,3 order before sorting.
	if migrations[0].ID != expectedSecond {
		t.Errorf("Expected migrations[0].ID = '%s' before sorting. Got '%s'.", expectedSecond, migrations[0].ID)
	}
	if migrations[1].ID != expectedFirst {
		t.Errorf("Expected migrations[1].ID = '%s' before sorting. Got '%s'.", expectedFirst, migrations[1].ID)
	}
	if migrations[2].ID != expectedThird {
		t.Errorf("Expected migrations[2].ID = '%s' before sorting. Got '%s'.", expectedThird, migrations[2].ID)
	}

	// Then we sort the slice
	SortMigrations(migrations)

	// Then we assert the order was changed as we expect
	if migrations[0].ID != expectedFirst {
		t.Errorf("Expected migrations[0].ID = '%s'. Got '%s'.", expectedFirst, migrations[0].ID)
	}
	if migrations[1].ID != expectedSecond {
		t.Errorf("Expected migrations[1].ID = '%s'. Got '%s'.", expectedSecond, migrations[1].ID)
	}
	if migrations[2].ID != expectedThird {
		t.Errorf("Expected migrations[2].ID = '%s'. Got '%s'.", expectedThird, migrations[2].ID)
	}
}

func expectID(t *testing.T, migration *Migration, expectedID string) {
	t.Helper()
	if migration.ID != expectedID {
		t.Errorf("Expected Migration to have ID '%s', got '%s' instead", expectedID, migration.ID)
	}
}

func expectScriptMatch(t *testing.T, migration *Migration, regexpString string) {
	t.Helper()
	re, err := regexp.Compile(regexpString)
	if err != nil {
		t.Fatalf("Invalid regexp: '%s': %s", regexpString, err)
	}
	if !re.MatchString(migration.Script) {
		t.Errorf("Expected migration Script to match '%s', but it did not. Script was:\n%s", regexpString, migration.Script)
	}
}
