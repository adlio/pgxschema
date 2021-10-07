package pgxschema

import "fmt"

// ErrNilDB is thrown when the database pointer is nil
var ErrNilDB = fmt.Errorf("Database connection is nil")
