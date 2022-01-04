package pgxschema

import (
	"context" // #nosec MD5 not being used cryptographically
	"fmt"
	"time"
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
}

// NewMigrator creates a new Migrator with the supplied
// options
func NewMigrator(options ...Option) *Migrator {
	m := Migrator{
		tableName: DefaultTableName,
		ctx:       context.Background(),
	}
	for _, opt := range options {
		m = opt(m)
	}
	m.lockID = LockIdentifierForTable(m.tableName)
	return &m
}

// QuotedTableName returns the dialect-quoted fully-qualified name for the
// migrations tracking table
func (m *Migrator) QuotedTableName() string {
	return QuotedTableName(m.schemaName, m.tableName)
}

// Apply takes a slice of Migrations and applies any which have not yet
// been applied
func (m *Migrator) Apply(db Connection, migrations []*Migration) error {
	if db == nil {
		return ErrNilDB
	}

	if len(migrations) == 0 {
		return nil
	}

	err := m.lock(db)
	if err != nil {
		return err
	}
	defer func() { err = coalesceErrs(err, m.unlock(db)) }()

	tx, err := db.Begin(m.ctx)
	if err != nil {
		return err
	}

	err = m.createMigrationsTable(tx)
	if err != nil {
		_ = tx.Rollback(m.ctx)
		return err
	}

	err = m.run(tx, migrations)
	if err != nil {
		_ = tx.Rollback(m.ctx)
		return err
	}

	err = tx.Commit(m.ctx)

	return err
}

func (m *Migrator) lock(db Queryer) error {
	query := fmt.Sprintf(`SELECT pg_advisory_lock(%d)`, m.lockID)
	_, err := db.Exec(m.ctx, query)
	if err == nil {
		m.log("Locked at ", time.Now().Format(time.RFC3339Nano))
	}
	return err
}

func (m *Migrator) createMigrationsTable(tx Queryer) error {
	tn := QuotedTableName(m.schemaName, m.tableName)
	query := fmt.Sprintf(`
				CREATE TABLE IF NOT EXISTS %s (
					id VARCHAR(255) NOT NULL,
					checksum VARCHAR(32) NOT NULL DEFAULT '',
					execution_time_in_millis INTEGER NOT NULL DEFAULT 0,
					applied_at TIMESTAMP WITH TIME ZONE NOT NULL
				)
			`, tn)
	_, err := tx.Exec(m.ctx, query)
	return err
}

func (m *Migrator) unlock(db Queryer) error {
	query := fmt.Sprintf(`SELECT pg_advisory_unlock(%d)`, m.lockID)
	_, err := db.Exec(m.ctx, query)
	if err == nil {
		m.log("Unlocked at ", time.Now().Format(time.RFC3339Nano))
	}
	return err
}

func (m *Migrator) run(tx Queryer, migrations []*Migration) error {
	if tx == nil {
		return ErrNilTx
	}

	plan, err := m.computeMigrationPlan(tx, migrations)
	if err != nil {
		return err
	}

	for _, migration := range plan {
		err := m.runMigration(tx, migration)
		if err != nil {
			return err
		}
	}

	return nil
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

func (m *Migrator) log(msgs ...interface{}) {
	if m.Logger != nil {
		m.Logger.Print(msgs...)
	}
}

func coalesceErrs(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
