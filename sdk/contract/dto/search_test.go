package dto

import (
	"reflect"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/go-admin-team/go-admin-core/v2/tools/search"
)

// GetIds used to append Id a second time inside an else branch: passing only
// Id returned [5 5], and a delete request deleted the same row twice.
func TestGeneralDelDtoGetIds(t *testing.T) {
	cases := []struct {
		name string
		dto  GeneralDelDto
		want []int
	}{
		{"id only", GeneralDelDto{Id: 5}, []int{5}},
		{"ids only", GeneralDelDto{Ids: []int{1, 2}}, []int{1, 2}},
		{"id and ids both set", GeneralDelDto{Id: 5, Ids: []int{1, 2}}, []int{5, 1, 2}},
		{"non-positive ids filtered out", GeneralDelDto{Ids: []int{0, -1, 3}}, []int{3}},
		{"zero id treated as unset", GeneralDelDto{Id: 0, Ids: []int{7}}, []int{7}},
		{"nothing set falls back to 0 (delete-all convention)", GeneralDelDto{}, []int{0}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.dto.GetIds()
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("GetIds() = %v, want %v", got, c.want)
			}
		})
	}
}

// nameDialector wraps a real, connectable dialector and reports a different
// Name(), so a test can exercise MakeCondition's Postgres branch without a
// live PostgreSQL server: the SQL syntax MakeCondition chooses depends only
// on what db.Dialector.Name() returns for that particular *gorm.DB.
type nameDialector struct {
	gorm.Dialector
	name string
}

func (d nameDialector) Name() string { return d.name }

func openWithDialectName(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(nameDialector{Dialector: sqlite.Open(":memory:"), name: name}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite under dialect name %q: %v", name, err)
	}
	return db
}

type searchProbe struct {
	Name string `search:"type:contains;column:name;table:probe"`
}

// icontainsProbe uses the case-insensitive variant, which is the one whose
// SQL actually differs between dialects (icontains -> ILIKE on Postgres,
// contains and icontains both fold to LIKE elsewhere).
type icontainsProbe struct {
	Name string `search:"type:icontains;column:name;table:probe"`
}

func statementSQL(t *testing.T, db *gorm.DB, q interface{}) string {
	t.Helper()
	tx := db.Session(&gorm.Session{DryRun: true}).Table("probe").Scopes(MakeCondition(q)).Find(&[]searchProbe{})
	if tx.Error != nil {
		t.Fatalf("build statement: %v", tx.Error)
	}
	return tx.Statement.SQL.String()
}

// TestMakeConditionReadsDialectFromDB is what F2 replaces global.Driver with:
// the closure asks the *gorm.DB it is handed, not a package-level variable,
// so two calls against two differently-dialected connections in the same
// process branch correctly at the same time.
func TestMakeConditionReadsDialectFromDB(t *testing.T) {
	q := searchProbe{Name: "abc"}

	mysqlSQL := statementSQL(t, openWithDialectName(t, search.Mysql), q)
	if !strings.Contains(strings.ToLower(mysqlSQL), "like") || strings.Contains(strings.ToLower(mysqlSQL), "ilike") {
		t.Fatalf("mysql-dialect SQL should use LIKE, not ILIKE: %s", mysqlSQL)
	}
	if !strings.Contains(mysqlSQL, "`probe`.`name`") {
		t.Fatalf("mysql-dialect SQL should backtick-quote the column: %s", mysqlSQL)
	}

	pgSQL := statementSQL(t, openWithDialectName(t, search.Postgres), icontainsProbe{Name: "abc"})
	if !strings.Contains(strings.ToLower(pgSQL), "ilike") {
		t.Fatalf("postgres-dialect SQL should use ILIKE: %s", pgSQL)
	}
	// MakeCondition itself never backtick-quotes the Postgres branch's WHERE
	// clause (unlike the FROM clause, which GORM's own quoter renders and is
	// out of MakeCondition's control).
	if strings.Contains(pgSQL, "`probe`.`name`") {
		t.Fatalf("postgres-dialect WHERE clause should not backtick-quote the column: %s", pgSQL)
	}

	// A driver name search has no constant for (sqlite) must fall back to the
	// MySQL-flavoured branch, matching today's behaviour with an unset
	// global.Driver.
	sqliteSQL := statementSQL(t, openWithDialectName(t, "sqlite"), q)
	if !strings.Contains(sqliteSQL, "`probe`.`name`") {
		t.Fatalf("sqlite (unknown dialect) should fall back to the MySQL-flavoured branch: %s", sqliteSQL)
	}
}

func TestPaginate(t *testing.T) {
	db := openWithDialectName(t, search.Mysql)
	tx := db.Session(&gorm.Session{DryRun: true}).Table("probe").Scopes(Paginate(20, 3)).Find(&[]searchProbe{})
	if tx.Error != nil {
		t.Fatalf("build statement: %v", tx.Error)
	}
	sql := tx.Statement.SQL.String()
	// GORM renders LIMIT/OFFSET as literals in the SQL text rather than as
	// bound placeholders, so the values are asserted against the string.
	if !strings.Contains(sql, "LIMIT 20") {
		t.Fatalf("expected LIMIT 20, got: %s", sql)
	}
	// page 3, size 20 -> offset 40
	if !strings.Contains(sql, "OFFSET 40") {
		t.Fatalf("expected OFFSET 40, got: %s", sql)
	}
}
