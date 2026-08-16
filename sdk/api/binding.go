package api

import (
	"github.com/gin-gonic/gin/binding"
	"reflect"
	"strings"
	"sync"
)

const (
	_ uint8 = iota
	json
	xml
	yaml
	form
	query
)

var constructor = &bindConstructor{}

// bindingOrder 固定 body 类 binding 的执行顺序：json/xml/yaml/form/query 在前，uri（nil binding）恒定放最后。
// 根因：gin 每个 binding 阶段都会跑一次完整 struct validator。若 uri 阶段先于 json，
// 则 json 字段（此时尚未填充、为零值）的 binding:"required" 会被 validator 误判失败。
// 旧实现用 map[uint8]binding.Binding + for range 返回，map 遍历顺序随机，
// 导致所有「同含 uri+json 且 json 字段有 required」的 PUT/POST 接口约 50% 概率间歇性参数校验失败。
var bindingOrder = []uint8{json, xml, yaml, form, query}

// The cache holds the finished []binding.Binding rather than the raw []uint8
// from resolveType: the raw form contains many duplicates once embedded structs
// are walked, so caching it would mean de-duplicating on every request. Keys are
// reflect.Type, not its string form, so identically named types in different
// packages cannot collide.
type bindConstructor struct {
	cache sync.Map // reflect.Type → []binding.Binding
}

func (e *bindConstructor) GetBindingForGin(d interface{}) []binding.Binding {
	t := reflect.TypeOf(d)
	if v, ok := e.cache.Load(t); ok {
		return v.([]binding.Binding)
	}

	gbs := e.build(t)
	e.cache.Store(t, gbs)
	return gbs
}

// build resolves a type into a de-duplicated, ordered binder list. It runs only
// on a cache miss.
func (e *bindConstructor) build(t reflect.Type) []binding.Binding {
	bs := e.resolveType(t.Elem())

	seen := make(map[uint8]bool, len(bs))
	for _, b := range bs {
		seen[b] = true
	}

	gbs := make([]binding.Binding, 0, len(seen))
	for _, b := range bindingOrder {
		if !seen[b] {
			continue
		}
		switch b {
		case json:
			gbs = append(gbs, binding.JSON)
		case xml:
			gbs = append(gbs, binding.XML)
		case yaml:
			gbs = append(gbs, binding.YAML)
		case form:
			gbs = append(gbs, binding.Form)
		case query:
			gbs = append(gbs, binding.Query)
		}
	}
	if seen[0] { // uri tag → nil binding，恒定放最后，确保 body 先绑定填充再做 uri
		gbs = append(gbs, nil)
	}
	return gbs
}

// resolveElem unwraps pointers, slices, arrays and maps, then resolves the
// struct type underneath.
func (e *bindConstructor) resolveElem(t reflect.Type) []uint8 {
	for {
		switch t.Kind() {
		case reflect.Ptr, reflect.Slice, reflect.Array, reflect.Map:
			t = t.Elem()
		case reflect.Struct:
			return e.resolveType(t)
		default:
			return nil
		}
	}
}

// resolveType resolves binders from a type; shared by resolve and by the
// recursion into embedded structs.
func (e *bindConstructor) resolveType(qType reflect.Type) []uint8 {
	bs := make([]uint8, 0)
	var tag reflect.StructTag
	var ok bool

	for i := 0; i < qType.NumField(); i++ {
		field := qType.Field(i)

		// Fields of an anonymously embedded struct bind as if they were declared
		// on the outer struct, so their tags must be resolved too. Generated
		// search DTOs have exactly this shape: pagination and ordering are
		// grouped into embedded structs, so without recursion a DTO with no
		// direct fields resolves to an empty binder list and pageIndex and the
		// ordering parameters are silently ignored (issue #72).
		if field.Anonymous {
			bs = append(bs, e.resolveElem(field.Type)...)
		}

		tag = field.Tag
		if _, ok = tag.Lookup("json"); ok {
			bs = append(bs, json)
		}
		if _, ok = tag.Lookup("xml"); ok {
			bs = append(bs, xml)
		}
		if _, ok = tag.Lookup("yaml"); ok {
			bs = append(bs, yaml)
		}
		if _, ok = tag.Lookup("form"); ok {
			bs = append(bs, form)
		}
		if _, ok = tag.Lookup("query"); ok {
			bs = append(bs, query)
		}
		if _, ok = tag.Lookup("uri"); ok {
			bs = append(bs, 0)
		}
		// dive means the element type must be resolved as well. The previous
		// implementation passed a reflect.Value into resolve, which then called
		// reflect.TypeOf(d).Elem() on it — that operates on the reflect.Value
		// struct itself and panics, so this branch never worked. Recurse on the
		// type instead.
		if t, ok := tag.Lookup("binding"); ok && strings.Contains(t, "dive") {
			bs = append(bs, e.resolveElem(field.Type)...)
			continue
		}
		if t, ok := tag.Lookup("validate"); ok && strings.Contains(t, "dive") {
			bs = append(bs, e.resolveElem(field.Type)...)
		}
	}
	return bs
}
