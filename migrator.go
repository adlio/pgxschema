package pgxschema

import (
	"context"
	"crypto/md5" // #nosec MD5 not being used cryptographically
	"fmt"
	"hash/crc32"
	"time"

	"github.com/jackc/pgx/v4"
)

const postgresAdvisoryLockSalt uint32 = 542384964

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
	return m.transaction(db, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, CreateSQL(QuotedTableName(m.SchemaName, m.TableName)))
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
	_, err = tx.Exec(
		m.ctx,
		InsertSQL(QuotedTableName(m.SchemaName, m.TableName)),
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
