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

type bindConstructor struct {
	cache map[string][]uint8
	mux   sync.Mutex
}

func (e *bindConstructor) GetBindingForGin(d interface{}) []binding.Binding {
	name := reflect.TypeOf(d).String()
	bs := e.getBinding(name)
	if bs == nil {
		//重新构建
		bs = e.resolve(d)
		e.setBinding(name, bs) // 顺手修复：旧实现 resolve 后从未写缓存，每次请求都重复 reflect
	}
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

func (e *bindConstructor) resolve(d interface{}) []uint8 {
	bs := make([]uint8, 0)
	qType := reflect.TypeOf(d).Elem()
	var tag reflect.StructTag
	var ok bool

	for i := 0; i < qType.NumField(); i++ {
		tag = qType.Field(i).Tag
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
		if t, ok := tag.Lookup("binding"); ok && strings.Index(t, "dive") > -1 {
			qValue := reflect.ValueOf(d)
			bs = append(bs, e.resolve(qValue.Field(i))...)
			continue
		}
		if t, ok := tag.Lookup("validate"); ok && strings.Index(t, "dive") > -1 {
			qValue := reflect.ValueOf(d)
			bs = append(bs, e.resolve(qValue.Field(i))...)
		}
	}
	return bs
}

func (e *bindConstructor) getBinding(name string) []uint8 {
	e.mux.Lock()
	defer e.mux.Unlock()
	return e.cache[name]
}

func (e *bindConstructor) setBinding(name string, bs []uint8) {
	e.mux.Lock()
	defer e.mux.Unlock()
	if e.cache == nil {
		e.cache = make(map[string][]uint8)
	}
	e.cache[name] = bs
}
