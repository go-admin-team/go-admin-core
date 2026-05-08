package search

import (
	"fmt"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

type ApplicationQuery struct {
	Id       string    `search:"type:icontains;column:id;table:receipt" form:"id"`
	Domain   string    `search:"type:icontains;column:domain;table:receipt" form:"domain"`
	Version  string    `search:"type:exact;column:version;table:receipt" form:"version"`
	Status   []int     `search:"type:in;column:status;table:receipt" form:"status"`
	Start    time.Time `search:"type:gte;column:created_at;table:receipt" form:"start"`
	End      time.Time `search:"type:lte;column:created_at;table:receipt" form:"end"`
	TestJoin `search:"type:left;on:id:receipt_id;table:receipt_goods;join:receipts"`
	NotNeed  string `search:"-"`
	ApplicationOrder
}

type ApplicationOrder struct {
	IdOrder string `search:"type:order;column:id;table:receipt" form:"id_order"`
}

type TestJoin struct {
	PaymentAccount string `search:"type:icontains;column:payment_account;table:receipts" form:"payment_account"`
}

func TestResolveSearchQuery(t *testing.T) {
	// Only pass t into top-level Convey calls
	Convey("Given some integer with a starting value", t, func() {
		d := ApplicationQuery{
			Id:               "aaa",
			Domain:           "bbb",
			Version:          "ccc",
			Status:           []int{1, 2},
			Start:            time.Now().Add(-8 * time.Hour),
			End:              time.Now(),
			ApplicationOrder: ApplicationOrder{IdOrder: "desc"},
			TestJoin:         TestJoin{PaymentAccount: "1212"},
		}
		condition := &GormCondition{
			GormPublic: GormPublic{},
			Join:       make([]*GormJoin, 0),
		}
		ResolveSearchQuery("mysql", d, condition)
		fmt.Println(condition)
	})
}

// JsonContainsQuery 用于覆盖 json_contains / json_contains_or_null 两种 type
type JsonContainsQuery struct {
	// json_contains：精确包含
	AgentCodeStrict string `search:"type:json_contains;column:agent_codes;table:tool_script_templates" form:"agentCodeStrict"`
	// json_contains_or_null：三态白名单
	AgentCodeWhitelist string `search:"type:json_contains_or_null;column:agent_codes;table:tool_script_templates" form:"agentCodeWhitelist"`
	// 与其他 type 共存校验
	Name string `search:"type:exact;column:name;table:tool_script_templates" form:"name"`
}

func TestJsonContains_MySQLZeroValueSkip(t *testing.T) {
	Convey("零值字段跳过：所有 string 都为空，不应生成任何 WHERE", t, func() {
		q := JsonContainsQuery{}
		cond := &GormCondition{GormPublic: GormPublic{}, Join: make([]*GormJoin, 0)}
		ResolveSearchQuery(Mysql, q, cond)

		So(len(cond.Where), ShouldEqual, 0)
	})
}

func TestJsonContains_MySQLSQLCorrectness(t *testing.T) {
	Convey("MySQL 分支：json_contains 生成 JSON_CONTAINS(col, JSON_QUOTE(?))", t, func() {
		q := JsonContainsQuery{AgentCodeStrict: "ai-coder"}
		cond := &GormCondition{GormPublic: GormPublic{}, Join: make([]*GormJoin, 0)}
		ResolveSearchQuery(Mysql, q, cond)

		expected := "JSON_CONTAINS(`tool_script_templates`.`agent_codes`, JSON_QUOTE(?))"
		_, exist := cond.Where[expected]
		So(exist, ShouldBeTrue)
		So(cond.Where[expected], ShouldResemble, []interface{}{"ai-coder"})
	})

	Convey("MySQL 分支：json_contains_or_null 生成三态 SQL", t, func() {
		q := JsonContainsQuery{AgentCodeWhitelist: "ai-reviewer"}
		cond := &GormCondition{GormPublic: GormPublic{}, Join: make([]*GormJoin, 0)}
		ResolveSearchQuery(Mysql, q, cond)

		expected := "(`tool_script_templates`.`agent_codes` IS NULL OR " +
			"`tool_script_templates`.`agent_codes` = JSON_ARRAY() OR " +
			"JSON_CONTAINS(`tool_script_templates`.`agent_codes`, JSON_QUOTE(?)))"
		_, exist := cond.Where[expected]
		So(exist, ShouldBeTrue)
		So(cond.Where[expected], ShouldResemble, []interface{}{"ai-reviewer"})
	})
}

func TestJsonContains_PostgresSQLCorrectness(t *testing.T) {
	Convey("Postgres 分支：json_contains 生成 col @> ?::jsonb", t, func() {
		q := JsonContainsQuery{AgentCodeStrict: "ai-coder"}
		cond := &GormCondition{GormPublic: GormPublic{}, Join: make([]*GormJoin, 0)}
		ResolveSearchQuery(Postgres, q, cond)

		expected := "tool_script_templates.agent_codes @> ?::jsonb"
		args, exist := cond.Where[expected]
		So(exist, ShouldBeTrue)
		// 标量字符串 → JSON 字面量 "ai-coder"
		So(args, ShouldResemble, []interface{}{`"ai-coder"`})
	})

	Convey("Postgres 分支：json_contains_or_null 生成三态 SQL", t, func() {
		q := JsonContainsQuery{AgentCodeWhitelist: "ai-reviewer"}
		cond := &GormCondition{GormPublic: GormPublic{}, Join: make([]*GormJoin, 0)}
		ResolveSearchQuery(Postgres, q, cond)

		expected := "(tool_script_templates.agent_codes IS NULL OR " +
			"tool_script_templates.agent_codes = '[]'::jsonb OR " +
			"tool_script_templates.agent_codes @> ?::jsonb)"
		args, exist := cond.Where[expected]
		So(exist, ShouldBeTrue)
		So(args, ShouldResemble, []interface{}{`"ai-reviewer"`})
	})
}

func TestJsonContains_CoexistWithOtherTypes(t *testing.T) {
	Convey("与 exact 等其他 type 共存：均生成 WHERE，互不冲突", t, func() {
		q := JsonContainsQuery{
			AgentCodeStrict:    "a1",
			AgentCodeWhitelist: "a2",
			Name:               "tpl-x",
		}
		cond := &GormCondition{GormPublic: GormPublic{}, Join: make([]*GormJoin, 0)}
		ResolveSearchQuery(Mysql, q, cond)

		So(len(cond.Where), ShouldEqual, 3)

		_, hasStrict := cond.Where["JSON_CONTAINS(`tool_script_templates`.`agent_codes`, JSON_QUOTE(?))"]
		So(hasStrict, ShouldBeTrue)

		_, hasWhitelist := cond.Where["(`tool_script_templates`.`agent_codes` IS NULL OR "+
			"`tool_script_templates`.`agent_codes` = JSON_ARRAY() OR "+
			"JSON_CONTAINS(`tool_script_templates`.`agent_codes`, JSON_QUOTE(?)))"]
		So(hasWhitelist, ShouldBeTrue)

		_, hasExact := cond.Where["`tool_script_templates`.`name` = ?"]
		So(hasExact, ShouldBeTrue)
	})
}

func TestJsonContains_BuildClauseHelpers(t *testing.T) {
	Convey("BuildJsonContainsClause MySQL", t, func() {
		sql, args := BuildJsonContainsClause(Mysql, "agent_codes", "ai-coder")
		So(sql, ShouldEqual, "JSON_CONTAINS(agent_codes, JSON_QUOTE(?))")
		So(args, ShouldResemble, []interface{}{"ai-coder"})
	})

	Convey("BuildJsonContainsClause Postgres", t, func() {
		sql, args := BuildJsonContainsClause(Postgres, "agent_codes", "ai-coder")
		So(sql, ShouldEqual, "agent_codes @> ?::jsonb")
		So(args, ShouldResemble, []interface{}{`"ai-coder"`})
	})

	Convey("BuildJsonContainsOrNullClause MySQL（白名单三态）", t, func() {
		sql, args := BuildJsonContainsOrNullClause(Mysql, "agent_codes", "ai-coder")
		So(sql, ShouldEqual, "(agent_codes IS NULL OR agent_codes = JSON_ARRAY() OR JSON_CONTAINS(agent_codes, JSON_QUOTE(?)))")
		So(args, ShouldResemble, []interface{}{"ai-coder"})
	})

	Convey("BuildJsonContainsOrNullClause Postgres（白名单三态）", t, func() {
		sql, args := BuildJsonContainsOrNullClause(Postgres, "agent_codes", "ai-coder")
		So(sql, ShouldEqual, "(agent_codes IS NULL OR agent_codes = '[]'::jsonb OR agent_codes @> ?::jsonb)")
		So(args, ShouldResemble, []interface{}{`"ai-coder"`})
	})

	Convey("BuildJsonContainsOrNullClause 支持表前缀（如 t.agent_codes）", t, func() {
		sql, args := BuildJsonContainsOrNullClause(Mysql, "t.agent_codes", "ai-coder")
		So(sql, ShouldEqual, "(t.agent_codes IS NULL OR t.agent_codes = JSON_ARRAY() OR JSON_CONTAINS(t.agent_codes, JSON_QUOTE(?)))")
		So(args, ShouldResemble, []interface{}{"ai-coder"})
	})

	Convey("默认驱动（空字符串）走 MySQL 语法", t, func() {
		sql, _ := BuildJsonContainsClause("", "agent_codes", "x")
		So(sql, ShouldEqual, "JSON_CONTAINS(agent_codes, JSON_QUOTE(?))")
	})
}
