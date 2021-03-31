package utils

import (
	"testing"
)

// TestIsNotExistMkDir 测试IsNotExistMkDir函数
func TestIsNotExistMkDir(t *testing.T) {
	err := IsNotExistMkDir("../../pkg/aaa")
	if err != nil {
		t.Error(err)
	} else {
		t.Log("done")
	}
}