package dto

import (
	"reflect"
	"testing"
)

func TestObjectByIdGetId(t *testing.T) {
	single := &ObjectById{Id: 5}
	if got := single.GetId(); got != 5 {
		t.Fatalf("GetId() = %v, want 5", got)
	}

	batch := &ObjectById{Id: 5, Ids: []int{1, 2}}
	got := batch.GetId()
	want := []int{1, 2, 5}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetId() = %v, want %v", got, want)
	}
}

func TestObjectGetReqGetId(t *testing.T) {
	req := &ObjectGetReq{Id: 9}
	if got := req.GetId(); got != 9 {
		t.Fatalf("GetId() = %v, want 9", got)
	}
}

func TestObjectDeleteReqGetId(t *testing.T) {
	req := &ObjectDeleteReq{Ids: []int{3, 4}}
	got := req.GetId()
	want := []int{3, 4}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetId() = %v, want %v", got, want)
	}
}
