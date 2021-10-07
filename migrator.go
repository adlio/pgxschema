package pgxschema

import (
	"context" // #nosec MD5 not being used cryptographically
	"fmt"
	"time"

	"github.com/jackc/pgx/v4"
)

// DefaultTableName defines the name of the database table which will
// hold the status of applied migrations
const DefaultTableName = "schema_migrations"

// Migrator is an instance customized to perform migrations on a particular
// against a particular tracking table and with a particular dialect
// defined.
type Migrator struct {
	// Logger provides an optional way for the Migrator to report status
	// messages. It is nil by default which results in no output.
	Logger Logger

	// schemaName is the Postgres schema where the schema_migrations table
	// will live. By default it will be blank, allowing the connection's
	// search_path to be leveraged. It can be set at creation via the first
	// argument to the WithTableName() option.
	schemaName string

	// tableName is the name of the table where the applied migrations will be
	// persisted. Unlike SchemaName, this can't be blank. If not provided via an
	// option, the DefaultTableName (schema_migrations) will be used instead.
	tableName string

	// lockID is the identifier for the Postgres global advisory lock
	// this value is computed from the TableName when the migrator is created
	lockID int64

	// ctx holds the context in which the migrator is running.
	ctx context.Context

	// err holds the last error which occurred at any step of the migration
	// process
	err error
}

// NewMigrator creates a new Migrator with the supplied
// options
func NewMigrator(options ...Option) Migrator {
	m := Migrator{
		tableName: DefaultTableName,
		ctx:       context.Background(),
	}
	for _, opt := range options {
		m = opt(m)
	}
	m.lockID = LockIdentifierForTable(m.tableName)
	return m
}

// Apply takes a slice of Migrations and applies any which have not yet
// been applied
func (m *Migrator) Apply(db Connection, migrations []*Migration) error {
	if db == nil {
		return ErrNilDB
	}

	m.err = nil
	tx := m.beginTx(db)
	m.lock(tx)
	defer m.unlock(tx)           // ... ensure we unlock even if errors occurred
	defer m.commitOrRollback(tx) // ... ensure we commit or rollback when done
	m.createMigrationsTable(tx)
	m.run(tx, migrations)
	return m.err
}

func (m *Migrator) beginTx(db Transactor) pgx.Tx {
	tx, err := db.Begin(m.ctx)
	if err != nil {
		m.err = fmt.Errorf("failed to begin transaction: %w", err)
	}
	return tx
}

func (m *Migrator) lock(db Queryer) {
	if m.err != nil {
		// Abort if Migrator already had an error
		return
	}
	query := fmt.Sprintf(`SELECT pg_advisory_lock(%d)`, m.lockID)
	_, err := db.Exec(m.ctx, query)
	if err == nil {
		m.log("Locked at ", time.Now().Format(time.RFC3339Nano))
	} else {
		m.err = err
	}
}

func (m *Migrator) createMigrationsTable(tx Queryer) {
	if m.err != nil {
		// Abort if Migrator already had an error
		return
	}
	tn := QuotedTableName(m.schemaName, m.tableName)
	query := fmt.Sprintf(`
				CREATE TABLE IF NOT EXISTS %s (
					id VARCHAR(255) NOT NULL,
					checksum VARCHAR(32) NOT NULL DEFAULT '',
					execution_time_in_millis INTEGER NOT NULL DEFAULT 0,
					applied_at TIMESTAMP WITH TIME ZONE NOT NULL
				)
			`, tn)
	_, m.err = tx.Exec(m.ctx, query)
}

func (m *Migrator) unlock(db Queryer) {
	query := fmt.Sprintf(`SELECT pg_advisory_unlock(%d)`, m.lockID)
	_, err := db.Exec(m.ctx, query)
	if err == nil {
		m.log("Unlocked at ", time.Now().Format(time.RFC3339Nano))
	} else if m.err == nil {
		// Don't overwrite an earlier error with an unlock error
		m.err = err
	}
}

func (m *Migrator) run(tx Queryer, migrations []*Migration) {
	if m.err != nil {
		// Abort if Migrator already had an error
		return
	}
	var plan []*Migration
	plan, m.err = m.computeMigrationPlan(tx, migrations)
	if m.err != nil {
		return
	}
	for _, migration := range plan {
		err := m.runMigration(tx, migration)
		if err != nil {
			m.err = err
			return
		}
	}
}

func (m *Migrator) computeMigrationPlan(db Queryer, toRun []*Migration) (plan []*Migration, err error) {
	applied, err := m.GetAppliedMigrations(db)
	if err != nil {
		return plan, err
	}
	plan = make([]*Migration, 0)
	for _, migration := range toRun {
		if _, exists := applied[migration.ID]; !exists {
			plan = append(plan, migration)
		}
	}
	SortMigrations(plan)
	return plan, err
}

func (m *Migrator) runMigration(tx Queryer, migration *Migration) error {
	startedAt := time.Now()
	_, err := tx.Exec(m.ctx, migration.Script)
	if err != nil {
		return fmt.Errorf("migration '%s' Failed: %w", migration.ID, err)
	}

	executionTime := time.Since(startedAt)
	m.log(fmt.Sprintf("Migration '%s' applied in %s\n", migration.ID, executionTime))

	tn := QuotedTableName(m.schemaName, m.tableName)
	query := fmt.Sprintf(`
				INSERT INTO %s
				( id, checksum, execution_time_in_millis, applied_at )
				VALUES
				( $1, $2, $3, $4 )
				`,
		tn,
	)
	_, err = tx.Exec(m.ctx, query, migration.ID, migration.MD5(), executionTime.Milliseconds(), startedAt)
	return err
}

func (m *Migrator) commitOrRollback(tx pgx.Tx) {
	if e := recover(); e != nil {
		switch e := e.(type) {
		case error:
			m.err = e
		default:
			m.err = fmt.Errorf("%s", e)
		}
	}
	if tx != nil {
		if m.err != nil {
			_ = tx.Rollback(m.ctx)
		} else {
			m.err = tx.Commit(m.ctx)
		}
	}
}

func (m *Migrator) log(msgs ...interface{}) {
	if m.Logger != nil {
		m.Logger.Print(msgs...)
	}
}
