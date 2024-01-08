package utils

import (
	"fmt"
	"reflect"

	"github.com/xuri/excelize/v2"
)

var Cols = []string{"", "A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R", "S", "T", "U", "V", "W", "X", "Y", "Z"}

// WriteXlsx 填充excel
func WriteXlsx(sheet string, records interface{}) *excelize.File {
	xlsx := excelize.NewFile()       // new file
	index, _ := xlsx.NewSheet(sheet) // new sheet
	xlsx.SetActiveSheet(index)       // set active (default) sheet
	t := reflect.TypeOf(records)

	if t.Kind() != reflect.Slice {
		return xlsx
	}

	s := reflect.ValueOf(records)
	for i := 0; i < s.Len(); i++ {
		elem := s.Index(i).Interface()
		elemType := reflect.TypeOf(elem)
		elemValue := reflect.ValueOf(elem)
		index := -1
		for j := 0; j < elemType.NumField(); j++ {
			field := elemType.Field(j)
			tag := field.Tag.Get("xlsx")
			name := tag
			if tag == "" || tag == "-" {
				continue
			}
			index++
			if index == -1 {
				continue
			}
			column, _ := ConvertNumToChars(index)
			// 设置表头
			if i == 0 {
				err := xlsx.SetCellValue(sheet, fmt.Sprintf("%s%d", column, i+1), name)
				if err != nil {
					return nil
				}
			}
			// 设置内容
			err := xlsx.SetCellValue(sheet, fmt.Sprintf("%s%d", column, i+2), elemValue.Field(j).Interface())
			if err != nil {
				return nil
			}
		}
	}
	return xlsx
}

func ConvertNumToChars(num int) (string, error) {
	var cols string
	v := num + 1
	for v > 0 {
		k := v % 26
		if k == 0 {
			k = 26
		}
		v = (v - k) / 26
		cols = Cols[k] + cols
	}
	return cols, nil
}
