package pgxschema

import (
	"context"
	"crypto/md5" // #nosec MD5 not being used cryptographically
	"fmt"
	"time"

	"github.com/jackc/pgx/v4"
)

// Migrator is an instance customized to perform migrations on a particular
// against a particular tracking table and with a particular dialect
// defined.
type Migrator struct {
	SchemaName string
	TableName  string
	Logger     Logger
	ctx        context.Context
}

// NewMigrator creates a new Migrator with the supplied
// options
func NewMigrator(options ...Option) Migrator {
	m := Migrator{
		TableName: DefaultTableName,
		ctx:       context.Background(),
	}
	for _, opt := range options {
		m = opt(m)
	}
	return m
}

// Apply takes a slice of Migrations and applies any which have not yet
// been applied
func (m Migrator) Apply(db Connection, migrations []*Migration) (err error) {
	err = m.lock(db)
	if err != nil {
		return err
	}

	defer func() {
		unlockErr := m.unlock(db)
		if unlockErr != nil {
			if err == nil {
				err = unlockErr
			} else {
				err = fmt.Errorf("error unlocking while returning from other err: %w\n%s", err, unlockErr.Error())
			}
		}
	}()

	err = m.createMigrationsTable(db)
	if err != nil {
		return err
	}

	err = m.transaction(db, func(ctx context.Context, tx pgx.Tx) error {
		applied, err := m.GetAppliedMigrations(tx)
		if err != nil {
			return err
		}

		plan := make([]*Migration, 0)
		for _, migration := range migrations {
			if _, exists := applied[migration.ID]; !exists {
				plan = append(plan, migration)
			}
		}

		SortMigrations(plan)

		for _, migration := range plan {
			err = m.runMigration(tx, migration)
			if err != nil {
				return err
			}
		}

		return nil
	})

	return err
}

// QuotedTableName returns the dialect-quoted fully-qualified name for the
// migrations tracking table
func (m Migrator) QuotedTableName() string {
	return QuotedTableName(m.SchemaName, m.TableName)
}

func (m Migrator) createMigrationsTable(db Transactor) (err error) {
	return m.transaction(db, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, CreateSQL(m.QuotedTableName()))
		return err
	})
}

func (m Migrator) lock(db Queryer) (err error) {
	if db == nil {
		return ErrNilDB
	}
	_, err = db.Exec(m.ctx, LockSQL(m.TableName))
	m.log("Locked at ", time.Now().Format(time.RFC3339Nano))
	return err
}

func (m Migrator) unlock(db Queryer) (err error) {
	if db == nil {
		return ErrNilDB
	}
	_, err = db.Exec(m.ctx, UnlockSQL(m.TableName))
	m.log("Unlocked at ", time.Now().Format(time.RFC3339Nano))
	return err
}

func (m Migrator) runMigration(tx Queryer, migration *Migration) error {
	var (
		err      error
		checksum string
	)

	startedAt := time.Now()
	_, err = tx.Exec(m.ctx, migration.Script)
	if err != nil {
		return fmt.Errorf("Migration '%s' Failed:\n%w", migration.ID, err)
	}

	executionTime := time.Since(startedAt)
	m.log(fmt.Sprintf("Migration '%s' applied in %s\n", migration.ID, executionTime))

	checksum = fmt.Sprintf("%x", md5.Sum([]byte(migration.Script))) // #nosec not using MD5 cryptographically
	_, err = tx.Exec(
		m.ctx,
		InsertSQL(m.QuotedTableName()),
		migration.ID,
		checksum,
		executionTime.Milliseconds(),
		startedAt,
	)
	return err
}

func (m Migrator) log(msgs ...interface{}) {
	if m.Logger != nil {
		m.Logger.Print(msgs...)
	}
}
