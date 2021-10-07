package pgxschema

import (
	"testing"

	"github.com/jackc/pgx/v4/pgxpool"
)

func TestGetAppliedMigrationsErrorsWhenNoneExist(t *testing.T) {
	withLatestDB(t, func(db *pgxpool.Pool) {
		migrator := makeTestMigrator()
		migrations, err := migrator.GetAppliedMigrations(db)
		if err == nil {

			t.Error("Expected an error. Got  none.")
		}
		if len(migrations) > 0 {
			t.Error("Expected empty list of applied migrations")
		}
	})
}
