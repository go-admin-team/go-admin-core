package utils

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"unsafe"
)

func Join(a []interface{}, sep string) string {
	if len(a) == 0 {
		return ""
	}
	if len(a) == 1 {
		return fmt.Sprintf("%v", a[0])
	}

	buffer := &bytes.Buffer{}
	buffer.WriteString(fmt.Sprintf("%v", a[0]))
	for i := 1; i < len(a); i++ {
		buffer.WriteString(sep)
		buffer.WriteString(fmt.Sprintf("%v", a[i]))
	}
	return buffer.String()
}

func ArrayToString(a []int, delim string) string {
	return strings.Trim(strings.Replace(fmt.Sprint(a), " ", delim, -1), "[]")
	//return strings.Trim(strings.Join(strings.Split(fmt.Sprint(a), " "), delim), "[]")
	//return strings.Trim(strings.Join(strings.Fields(fmt.Sprint(a)), delim), "[]")
}

func ReverseString(s string) string {
	chars := []rune(s)
	for i, j := 0, len(chars)-1; i < j; i, j = i+1, j-1 {
		chars[i], chars[j] = chars[j], chars[i]
	}
	return string(chars)
}

func ReverseWords(s string) string {
	words := strings.Fields(s)
	for i, j := 0, len(words)-1; i < j; i, j = i+1, j-1 {
		words[i], words[j] = words[j], words[i]
	}
	return strings.Join(words, " ")
}

// 不要在使用[]byte(string) 或 string([]byte)这类类型转换
// http://www.flysnow.org/2017/07/06/go-in-action-unsafe-pointer.html
// https://studygolang.com/articles/2909
// unsafe.Pointer 类似 void*
//
func Bytes2String(b []byte) string {
	// string 实际是reflect.StringHeader结构体和这个结构体所指向的内存
	// slice 实际是reflect.SliceHeader结构体和这个结构体中Data字段所指向的内存
	// slice 和 string reflect.StringHeader和reflect.SliceHeader的结构体只相差末尾一个字段，两者的内存是对其的，没必要再取Data字段了
	return *(*string)(unsafe.Pointer(&b))
}

func String2Bytes(s string) []byte {
	sh := (*[2]uintptr)(unsafe.Pointer(&s))
	// bh := reflect.SliceHeader{
	// 	Data: sh.Data,
	// 	Len:  sh.Len,
	// 	Cap:  sh.Len,
	// }
	bh := [3]uintptr{sh[0], sh[1], sh[1]}
	return *(*[]byte)(unsafe.Pointer(&bh))
}

// &s[0]
func StringPointer(s string) unsafe.Pointer {
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	return unsafe.Pointer(sh.Data)
}

// &b[0]
func BytesPointer(b []byte) unsafe.Pointer {
	bh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	return unsafe.Pointer(bh.Data)
}
