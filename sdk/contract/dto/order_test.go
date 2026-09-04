package dto

import (
	"strings"
	"testing"

	"gorm.io/gorm"

	"github.com/go-admin-team/go-admin-core/v2/tools/search"
)

func TestOrderDest(t *testing.T) {
	db := openWithDialectName(t, search.Mysql)

	asc := db.Session(&gorm.Session{DryRun: true}).Table("probe").Scopes(OrderDest("created_at", false)).Find(&[]searchProbe{})
	if !strings.Contains(asc.Statement.SQL.String(), "ORDER BY") {
		t.Fatalf("expected an ORDER BY clause, got: %s", asc.Statement.SQL.String())
	}
	if strings.Contains(asc.Statement.SQL.String(), "DESC") {
		t.Fatalf("bl=false should not produce DESC: %s", asc.Statement.SQL.String())
	}

	desc := db.Session(&gorm.Session{DryRun: true}).Table("probe").Scopes(OrderDest("created_at", true)).Find(&[]searchProbe{})
	if !strings.Contains(desc.Statement.SQL.String(), "DESC") {
		t.Fatalf("bl=true should produce DESC: %s", desc.Statement.SQL.String())
	}
}
