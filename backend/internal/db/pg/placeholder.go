package pg

import "database/sql"

// Placeholder types for future Postgres implementations.
type DB struct {
	Conn *sql.DB
}

func New(db *sql.DB) *DB {
	return &DB{Conn: db}
}
