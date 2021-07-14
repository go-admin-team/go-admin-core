package antd_apis

import (
	"fmt"
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

type bindConstructor struct {
	cache map[string][]uint8
	mux   sync.Mutex
}

func (e *bindConstructor) GetBindingForGin(d interface{}) []binding.Binding {
	bs := e.getBinding(reflect.TypeOf(d).String())
	if bs == nil {
		//重新构建
		bs = e.resolve(d)
	}
	gbs := make([]binding.Binding, 0)
	mp := make(map[uint8]binding.Binding, 0)
	for _, b := range bs {
		switch b {
		case json:
			mp[json] = binding.JSON
		case xml:
			mp[xml] = binding.XML
		case yaml:
			mp[yaml] = binding.YAML
		case form:
			mp[form] = binding.Form
		case query:
			mp[query] = binding.Query
		default:
			mp[0] = nil
		}
	}
	for e := range mp {
		gbs=append(gbs, mp[e])
	}
	return gbs
}

func (e *bindConstructor) resolve(d interface{}) []uint8 {
	bs := make([]uint8, 0)
	qType := reflect.TypeOf(d).Elem()
	var tag reflect.StructTag
	var ok bool
	fmt.Println(qType.Kind())
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

