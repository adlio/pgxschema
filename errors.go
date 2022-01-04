package pgxschema

import "fmt"

// ErrNilDB is thrown when the database pointer is nil
var ErrNilDB = fmt.Errorf("Database connection is nil")

// ErrNilTx is thrown when a command is run against a nil transaction
var ErrNilTx = fmt.Errorf("Database transaction is nil")
