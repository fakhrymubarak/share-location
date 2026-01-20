package pg

import (
	"database/sql"
	"testing"
)

func TestNewDBWrapper(t *testing.T) {
	raw := &sql.DB{}
	db := New(raw)
	if db.Conn != raw {
		t.Fatalf("expected wrapped db")
	}
}
