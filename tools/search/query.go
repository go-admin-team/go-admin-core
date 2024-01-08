package search

import (
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
		tag, ok = "", false
		tag, ok = qType.Field(i).Tag.Lookup(FromQueryTag)
		if !ok {
			//递归调用
			ResolveSearchQuery(driver, qValue.Field(i).Interface(), condition)
			continue
		}
		switch tag {
		case "-":
			continue
		}
		t = makeTag(tag)
		if qValue.Field(i).IsZero() {
			continue
		}
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
			condition.SetWhere(fmt.Sprintf("%s.%s isnull", t.Table, t.Column), make([]interface{}, 0))
		}
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
			condition.SetWhere(fmt.Sprintf("`%s`.`%s` isnull", t.Table, t.Column), make([]interface{}, 0))
		}
	case "order":
		switch strings.ToLower(qValue.Field(i).String()) {
		case "desc", "asc":
			condition.SetOrder(fmt.Sprintf("`%s`.`%s` %s", t.Table, t.Column, qValue.Field(i).String()))
		}
	}
}
