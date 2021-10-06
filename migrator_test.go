package pgxschema

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v4"
)

func TestApplyWithNilDBProvidesHelpfulError(t *testing.T) {
	err := NewMigrator().Apply(nil, []*Migration{
		{
			ID:     "2019-01-01 Test",
			Script: "CREATE TABLE fake_table (id INTEGER)",
		},
	})
	if !errors.Is(err, ErrNilDB) {
		t.Errorf("Expected %v, got %v", ErrNilDB, err)
	}
}
func TestMigrationRecoversFromPanics(t *testing.T) {
	db := connectDB(t, "postgres11")
	m := NewMigrator()
	err := m.transaction(db, func(ctx context.Context, tx pgx.Tx) error {
		panic(errors.New("Panic Error"))
	})
	if err.Error() != "Panic Error" {
		t.Errorf("Expected panic to be converted to error=Panic Error. Got %v", err)
	}
}
