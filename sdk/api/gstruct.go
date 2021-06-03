package api

import (
	"fmt"
	"github.com/gin-gonic/gin/binding"
	"reflect"
)

const (
	form string = "form"
	xml  string = "xml"
	json string = "json"
	yaml string = "yaml"
	uri  string = "uri"
)

func Resolve(q interface{}) []binding.Binding {
	bindings :=make([]binding.Binding,0)
	qType := reflect.TypeOf(q)
	qValue := reflect.ValueOf(q)
	fmt.Println("qType", qType)
	fmt.Println("qValue", qValue)
	fmt.Println("qType.NumField", qType.NumField())
	fmt.Println("qValue.NumField", qValue.NumField())
	var okForm, okJson, okUri, okYaml, okXml bool
	var mp = make(map[string]binding.Binding, 0)
	for i := 0; i < qType.NumField(); i++ {
		okForm, okJson, okUri, okYaml, okXml = false, false, false, false, false
		tag := qType.Field(i).Tag
		fmt.Println("tag", tag)
		if _, okForm = tag.Lookup(form); okForm {
			mp[form] = binding.Form
		}
		if _, okJson = tag.Lookup(json); okJson {
			mp[json] = binding.JSON
		}
		if _, okUri = tag.Lookup(uri); okUri {
			mp[uri] = nil
		}
		if _, okYaml = tag.Lookup(yaml); okYaml {
			mp[yaml] = binding.YAML
		}
		if _, okXml = tag.Lookup(xml); okXml {
			mp[xml] = binding.XML
		}

		if !okForm && !okJson && !okUri && !okYaml && !okXml {
			//递归调用
			qList := Resolve(qValue.Field(i).Interface())
			for _, b := range qList {
				bindings = append(bindings, b)
			}
			continue
		}
	}
	for s := range mp {
		bindings = append(bindings, mp[s])
	}
	return bindings
}
