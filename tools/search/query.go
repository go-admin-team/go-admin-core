package search

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

const (
	// FromQueryTag tag标记
	FromQueryTag = "search"
	// Mysql 数据库标识
	Mysql = "mysql"
	// Postgres 数据库标识
	Postgres = "postgres"
)

// ResolveSearchQuery 解析
/**
 * 	exact / iexact 等于
 * 	contains / icontains 包含
 *	gt / gte 大于 / 大于等于
 *	lt / lte 小于 / 小于等于
 *	startswith / istartswith 以…起始
 *	endswith / iendswith 以…结束
 *	in
 *	isnull
 *	json_contains          JSON 数组精确包含值（MySQL JSON_CONTAINS / Postgres @>）
 *	json_contains_or_null  JSON 数组三态语义：NULL / 空数组 → 全部命中；非空 → 按值匹配（白名单语义）
 *  order 排序		e.g. order[key]=desc     order[key]=asc
 */
func ResolveSearchQuery(driver string, q interface{}, condition Condition) {
	qType := reflect.TypeOf(q)
	qValue := reflect.ValueOf(q)
	var tag string
	var ok bool
	var t *resolveSearchTag

	//var sep = "`"
	//if driver == Postgres {
	//	sep = "\""
	//}

	for i := 0; i < qType.NumField(); i++ {
		// Reflection cannot read an unexported field, and Interface panics on
		// one rather than returning an error - so a single unexported field on
		// a search DTO took down the whole list request. A field the author did
		// not export is not part of the search contract either way.
		field := qType.Field(i)
		if !field.IsExported() {
			continue
		}

		tag, ok = "", false
		tag, ok = field.Tag.Lookup(FromQueryTag)
		if !ok {
			//递归调用
			ResolveSearchQuery(driver, qValue.Field(i).Interface(), condition)
			continue
		}
		switch tag {
		case "-":
			continue
		}
		// The zero check comes before the tag is parsed, not after. A list page
		// opened without filters leaves every field zero, and makeTag allocates
		// on each one only for the result to be dropped on the next line - two
		// thirds of the work on the most common request of all.
		if qValue.Field(i).IsZero() {
			continue
		}
		t = makeTag(tag)
		//解析 Postgres `语法不支持，单独适配
		if driver == Postgres {
			pgSql(driver, t, condition, qValue, i)
		} else {
			otherSql(driver, t, condition, qValue, i)
		}
	}
}

func pgSql(driver string, t *resolveSearchTag, condition Condition, qValue reflect.Value, i int) {
	switch t.Type {
	case "left":
		//左关联
		join := condition.SetJoinOn(t.Type, fmt.Sprintf(
			"left join %s on %s.%s = %s.%s", t.Join, t.Join, t.On[0], t.Table, t.On[1],
		))
		ResolveSearchQuery(driver, qValue.Field(i).Interface(), join)
	case "exact", "iexact":
		condition.SetWhere(fmt.Sprintf("%s.%s = ?", t.Table, t.Column), []interface{}{qValue.Field(i).Interface()})
	case "glt":
		condition.SetWhere(fmt.Sprintf("%s.%s <> ?", t.Table, t.Column), []interface{}{qValue.Field(i).Interface()})
	case "icontains":
		condition.SetWhere(fmt.Sprintf("%s.%s ilike ?", t.Table, t.Column), []interface{}{"%" + qValue.Field(i).String() + "%"})
	case "contains":
		condition.SetWhere(fmt.Sprintf("%s.%s like ?", t.Table, t.Column), []interface{}{"%" + qValue.Field(i).String() + "%"})
	case "gt":
		condition.SetWhere(fmt.Sprintf("%s.%s > ?", t.Table, t.Column), []interface{}{qValue.Field(i).Interface()})
	case "gte":
		condition.SetWhere(fmt.Sprintf("%s.%s >= ?", t.Table, t.Column), []interface{}{qValue.Field(i).Interface()})
	case "lt":
		condition.SetWhere(fmt.Sprintf("%s.%s < ?", t.Table, t.Column), []interface{}{qValue.Field(i).Interface()})
	case "lte":
		condition.SetWhere(fmt.Sprintf("%s.%s <= ?", t.Table, t.Column), []interface{}{qValue.Field(i).Interface()})
	case "istartswith":
		condition.SetWhere(fmt.Sprintf("%s.%s ilike ?", t.Table, t.Column), []interface{}{qValue.Field(i).String() + "%"})
	case "startswith":
		condition.SetWhere(fmt.Sprintf("%s.%s like ?", t.Table, t.Column), []interface{}{qValue.Field(i).String() + "%"})
	case "iendswith":
		condition.SetWhere(fmt.Sprintf("%s.%s ilike ?", t.Table, t.Column), []interface{}{"%" + qValue.Field(i).String()})
	case "endswith":
		condition.SetWhere(fmt.Sprintf("%s.%s like ?", t.Table, t.Column), []interface{}{"%" + qValue.Field(i).String()})
	case "in":
		condition.SetWhere(fmt.Sprintf("%s.%s in (?)", t.Table, t.Column), []interface{}{qValue.Field(i).Interface()})
	case "isnull":
		if !(qValue.Field(i).IsZero() && qValue.Field(i).IsNil()) {
			condition.SetWhere(fmt.Sprintf("%s.%s is null", t.Table, t.Column), make([]interface{}, 0))
		}
	case "json_contains":
		// Postgres：col @> ?::jsonb；参数序列化成 JSON 字符串作为 jsonb cast 的输入
		condition.SetWhere(
			fmt.Sprintf("%s.%s @> ?::jsonb", t.Table, t.Column),
			[]interface{}{jsonEncodeValue(qValue.Field(i).Interface())},
		)
	case "json_contains_or_null":
		// Postgres 三态：NULL / '[]'::jsonb / col @> ?::jsonb
		condition.SetWhere(
			fmt.Sprintf("(%s.%s IS NULL OR %s.%s = '[]'::jsonb OR %s.%s @> ?::jsonb)",
				t.Table, t.Column, t.Table, t.Column, t.Table, t.Column),
			[]interface{}{jsonEncodeValue(qValue.Field(i).Interface())},
		)
	case "order":
		switch strings.ToLower(qValue.Field(i).String()) {
		case "desc", "asc":
			condition.SetOrder(fmt.Sprintf("%s.%s %s", t.Table, t.Column, qValue.Field(i).String()))
		}
	}
}

func otherSql(driver string, t *resolveSearchTag, condition Condition, qValue reflect.Value, i int) {
	switch t.Type {
	case "left":
		//左关联
		join := condition.SetJoinOn(t.Type, fmt.Sprintf(
			"left join `%s` on `%s`.`%s` = `%s`.`%s`",
			t.Join,
			t.Join,
			t.On[0],
			t.Table,
			t.On[1],
		))
		ResolveSearchQuery(driver, qValue.Field(i).Interface(), join)
	case "exact", "iexact":
		condition.SetWhere(fmt.Sprintf("`%s`.`%s` = ?", t.Table, t.Column), []interface{}{qValue.Field(i).Interface()})
	case "contains", "icontains":
		condition.SetWhere(fmt.Sprintf("`%s`.`%s` like ?", t.Table, t.Column), []interface{}{"%" + qValue.Field(i).String() + "%"})
	case "gt":
		condition.SetWhere(fmt.Sprintf("`%s`.`%s` > ?", t.Table, t.Column), []interface{}{qValue.Field(i).Interface()})
	case "gte":
		condition.SetWhere(fmt.Sprintf("`%s`.`%s` >= ?", t.Table, t.Column), []interface{}{qValue.Field(i).Interface()})
	case "lt":
		condition.SetWhere(fmt.Sprintf("`%s`.`%s` < ?", t.Table, t.Column), []interface{}{qValue.Field(i).Interface()})
	case "lte":
		condition.SetWhere(fmt.Sprintf("`%s`.`%s` <= ?", t.Table, t.Column), []interface{}{qValue.Field(i).Interface()})
	case "startswith", "istartswith":
		condition.SetWhere(fmt.Sprintf("`%s`.`%s` like ?", t.Table, t.Column), []interface{}{qValue.Field(i).String() + "%"})
	case "endswith", "iendswith":
		condition.SetWhere(fmt.Sprintf("`%s`.`%s` like ?", t.Table, t.Column), []interface{}{"%" + qValue.Field(i).String()})
	case "in":
		condition.SetWhere(fmt.Sprintf("`%s`.`%s` in (?)", t.Table, t.Column), []interface{}{qValue.Field(i).Interface()})
	case "isnull":
		if !(qValue.Field(i).IsZero() && qValue.Field(i).IsNil()) {
			condition.SetWhere(fmt.Sprintf("`%s`.`%s` is null", t.Table, t.Column), make([]interface{}, 0))
		}
	case "json_contains":
		// MySQL/SQLite/SQL Server：JSON_CONTAINS(col, JSON_QUOTE(?))；标量值用 JSON_QUOTE 包裹成 JSON 字面量
		condition.SetWhere(
			fmt.Sprintf("JSON_CONTAINS(`%s`.`%s`, JSON_QUOTE(?))", t.Table, t.Column),
			[]interface{}{qValue.Field(i).Interface()},
		)
	case "json_contains_or_null":
		// MySQL 三态：NULL / JSON_ARRAY() / JSON_CONTAINS(col, JSON_QUOTE(?))；白名单语义
		condition.SetWhere(
			fmt.Sprintf("(`%s`.`%s` IS NULL OR `%s`.`%s` = JSON_ARRAY() OR JSON_CONTAINS(`%s`.`%s`, JSON_QUOTE(?)))",
				t.Table, t.Column, t.Table, t.Column, t.Table, t.Column),
			[]interface{}{qValue.Field(i).Interface()},
		)
	case "order":
		switch strings.ToLower(qValue.Field(i).String()) {
		case "desc", "asc":
			condition.SetOrder(fmt.Sprintf("`%s`.`%s` %s", t.Table, t.Column, qValue.Field(i).String()))
		}
	}
}

// jsonEncodeValue 将 Go 值序列化为 JSON 字面量（用于 Postgres jsonb cast）。
// 标量字符串 → "value"，数字 → 123，bool → true/false。失败时回退到 fmt.Sprintf 字符串化。
func jsonEncodeValue(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		// 兜底：包成 JSON 字符串字面量
		return fmt.Sprintf("%q", fmt.Sprint(v))
	}
	return string(b)
}

// BuildJsonContainsClause 构造"JSON 数组精确包含值"的 WHERE 子句和参数。
// 用于 Service 层函数式调用（非 DTO + search tag 路径）需要单 Service 手写白名单时复用。
//
// driver: "mysql" / "postgres" / 其他（默认走 MySQL 语法）
// column: 完整列引用，如 "agent_codes" 或 "t.agent_codes"
// value:  JSON 数组中要查找的标量值（通常是 string）
//
// 返回 (sqlClause, args) 可直接传 *gorm.DB.Where(sqlClause, args...)。
func BuildJsonContainsClause(driver, column string, value interface{}) (string, []interface{}) {
	if driver == Postgres {
		return fmt.Sprintf("%s @> ?::jsonb", column), []interface{}{jsonEncodeValue(value)}
	}
	return fmt.Sprintf("JSON_CONTAINS(%s, JSON_QUOTE(?))", column), []interface{}{value}
}

// BuildJsonContainsOrNullClause 构造"JSON 数组三态白名单语义"的 WHERE 子句和参数：
//   - 列为 NULL → 全部命中（视为不限制）
//   - 列为空数组 → 全部命中
//   - 列为非空数组 → 仅匹配值才命中
//
// 适合"白名单字段：未配置 = 全部允许；配置后 = 仅指定项允许"的最小权限模型。
//
// driver: "mysql" / "postgres" / 其他（默认走 MySQL 语法）
// column: 完整列引用，如 "agent_codes" 或 "t.agent_codes"
// value:  JSON 数组中要查找的标量值（通常是 string）
func BuildJsonContainsOrNullClause(driver, column string, value interface{}) (string, []interface{}) {
	if driver == Postgres {
		return fmt.Sprintf("(%s IS NULL OR %s = '[]'::jsonb OR %s @> ?::jsonb)", column, column, column),
			[]interface{}{jsonEncodeValue(value)}
	}
	return fmt.Sprintf("(%s IS NULL OR %s = JSON_ARRAY() OR JSON_CONTAINS(%s, JSON_QUOTE(?)))", column, column, column),
		[]interface{}{value}
}
