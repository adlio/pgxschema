package pgxschema

import (
	"context"
	"crypto/md5" // #nosec MD5 not being used cryptographically
	"fmt"
	"hash/crc32"
	"time"

	"github.com/jackc/pgx/v4"
)

// DefaultTableName defines the name of the database table which will
// hold the status of applied migrations
const DefaultTableName = "schema_migrations"

const postgresAdvisoryLockSalt uint32 = 542384964

// Migrator is an instance customized to perform migrations on a particular
// against a particular tracking table and with a particular dialect
// defined.
type Migrator struct {
	// SchemaName is the Postgres schema where the schema_migrations table
	// will live. By default it will be blank, allowing the connection's
	// search_path to be leveraged. It can be set at creation via the first
	// argument to the WithTableName() option.
	SchemaName string

	// TableName is the name of the table where the applied migrations will be
	// persisted. Unlike SchemaName, this can't be blank. If not provided via an
	// option, the DefaultTableName (schema_migrations) will be used instead.
	TableName string

	// Logger provides an optional way for the Migrator to report status
	// messages. It is nil by default which results in no output.
	Logger Logger

	// ctx holds the context in which the migrator is running
	ctx context.Context

	// err holds the last error which occurred at any step of the migration
	// process
	err error
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

// AdvisoryLockID computes a unique identifier for this migrator's global
// advisory lock. It's based on the migrator's TableName.
func (m Migrator) AdvisoryLockID() string {
	sum := crc32.ChecksumIEEE([]byte(m.TableName))
	sum = sum * postgresAdvisoryLockSalt
	return fmt.Sprint(sum)
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

func (m Migrator) createMigrationsTable(db Transactor) (err error) {
	tn := QuotedTableName(m.SchemaName, m.TableName)
	query := fmt.Sprintf(`
				CREATE TABLE IF NOT EXISTS %s (
					id VARCHAR(255) NOT NULL,
					checksum VARCHAR(32) NOT NULL DEFAULT '',
					execution_time_in_millis INTEGER NOT NULL DEFAULT 0,
					applied_at TIMESTAMP WITH TIME ZONE NOT NULL
				)
			`, tn)

	return m.transaction(db, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, query)
		return err
	})
}

func (m Migrator) lock(db Queryer) (err error) {
	if db == nil {
		return ErrNilDB
	}
	lockID := m.AdvisoryLockID()
	query := fmt.Sprintf(`SELECT pg_advisory_lock(%s)`, lockID)
	_, err = db.Exec(m.ctx, query)
	if err != nil {
		return err
	}
	m.log("Locked at ", time.Now().Format(time.RFC3339Nano))
	return nil
}

func (m Migrator) unlock(db Queryer) (err error) {
	if db == nil {
		return ErrNilDB
	}
	lockID := m.AdvisoryLockID()
	query := fmt.Sprintf(`SELECT pg_advisory_unlock(%s)`, lockID)
	_, err = db.Exec(m.ctx, query)
	if err != nil {
		return err
	}
	m.log("Unlocked at ", time.Now().Format(time.RFC3339Nano))
	return nil
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
	tn := QuotedTableName(m.SchemaName, m.TableName)
	query := fmt.Sprintf(`
				INSERT INTO %s
				( id, checksum, execution_time_in_millis, applied_at )
				VALUES
				( $1, $2, $3, $4 )
				`,
		tn,
	)
	_, err = tx.Exec(m.ctx, query, migration.ID, checksum, executionTime.Milliseconds(), startedAt)
	return err
}

func (m Migrator) log(msgs ...interface{}) {
	if m.Logger != nil {
		m.Logger.Print(msgs...)
	}
}

// transaction wraps the supplied function in a transaction with the supplied
// database connecion
//
func (m *Migrator) transaction(db Transactor, f func(context.Context, pgx.Tx) error) (err error) {
	if db == nil {
		return ErrNilDB
	}
	tx, err := db.Begin(m.ctx)
	if err != nil {
		return
	}

	defer func() {
		if p := recover(); p != nil {
			switch p := p.(type) {
			case error:
				err = p
			default:
				err = fmt.Errorf("%s", p)
			}
		}
		if err != nil {
			_ = tx.Rollback(m.ctx)
			return
		}
		err = tx.Commit(m.ctx)
	}()

	return f(m.ctx, tx)
}
