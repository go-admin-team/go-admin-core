package models

import (
	"reflect"
	"sort"
	"strconv"
	"sync"
	"testing"

	"gorm.io/gorm/schema"
)

// probeRow is the shape a hand-written model takes when it embeds this
// package's building blocks, mirroring the OldStyle/Current structs in
// tests/alias-spike/consumer/consumer_test.go so the two suites stay
// comparable.
type probeRow struct {
	Model
	Name string `json:"name" gorm:"size:64;comment:名称"`
	ControlBy
	ModelTime
}

func (probeRow) TableName() string         { return "spike_row" }
func (e *probeRow) GetId() interface{}     { return e.Id }
func (e *probeRow) Generate() ActiveRecord { o := *e; return &o }

// TestActiveRecordSelfReference checks that a model built from this
// package's embeds satisfies the self-referencing ActiveRecord interface
// (Generate() ActiveRecord). This is the shape alias-spike's reverse
// counterproof A breaks: swap the type alias go-admin/common/models keeps
// for a defined type, and an app's Generate() method — which still returns
// the app's own import of the interface — stops satisfying this one.
func TestActiveRecordSelfReference(t *testing.T) {
	var rec ActiveRecord = &probeRow{}
	if got := rec.TableName(); got != "spike_row" {
		t.Fatalf("TableName() = %q, want spike_row", got)
	}
	rec.SetCreateBy(7)
	rec.SetUpdateBy(8)
	generated := rec.Generate()
	if generated == rec {
		t.Fatal("Generate() returned the same pointer; callers that reuse a " +
			"prototype across concurrent requests need a copy, not the receiver")
	}
}

// wantSchema is the frozen contract: the field set every fork's `sys_*`
// tables have carried since before this package existed, captured as
// literals rather than derived from any package this test also compiles.
//
// This must be a hardcoded golden, not a comparison against another Go type
// in this repository. After the F1/F5 move, go-admin/common/models becomes a
// pure alias of this package — comparing this package's schema against that
// alias would compare a type against itself, which stays green no matter how
// the shape drifts (PRD 006 评审 B1). The literals below are what
// go-admin/common/models produced before the move, computed once via
// gorm/schema.Parse on the same embed combination as probeRow, and are not
// regenerated from this package's own types.
var wantSchema = []string{
	"create_by|int|pk=false|size=64",
	"created_at|time|pk=false|size=0",
	"deleted_at|uint|pk=false|size=64", // soft_delete.DeletedAt, not gorm.DeletedAt
	"id|int|pk=true|size=64",
	"name|string|pk=false|size=64",
	"update_by|int|pk=false|size=64",
	"updated_at|time|pk=false|size=0",
}

func describeSchema(t *testing.T, v interface{}) []string {
	t.Helper()
	s, err := schema.Parse(v, &sync.Map{}, schema.NamingStrategy{SingularTable: true})
	if err != nil {
		t.Fatalf("schema.Parse(%T): %v", v, err)
	}
	out := make([]string, 0, len(s.Fields))
	for _, f := range s.Fields {
		out = append(out, f.DBName+"|"+string(f.DataType)+"|pk="+strconv.FormatBool(f.PrimaryKey)+"|size="+strconv.Itoa(f.Size))
	}
	sort.Strings(out)
	return out
}

// TestSchemaMatchesFrozenGolden is validated by counterproof B (see the
// package doc comment in ctproof_test.go): swapping ModelTime.DeletedAt for
// gorm.DeletedAt changes deleted_at from uint to time and this test goes red.
func TestSchemaMatchesFrozenGolden(t *testing.T) {
	got := describeSchema(t, &probeRow{})
	if !reflect.DeepEqual(got, wantSchema) {
		t.Fatalf("contract schema drifted from the frozen golden\n want: %v\n got:  %v", wantSchema, got)
	}
}

// TestEmbeddedFieldNames locks the embed order reflect sees, since a
// consumer that reads struct tags or does field-name based reflection over
// its own model depends on it.
func TestEmbeddedFieldNames(t *testing.T) {
	want := []string{"Model", "Name", "ControlBy", "ModelTime"}
	rt := reflect.TypeOf(probeRow{})
	got := make([]string, 0, rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		got = append(got, rt.Field(i).Name)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("embedded field names = %v, want %v", got, want)
	}
}
