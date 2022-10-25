package jwtauth

import (
	"encoding/json"
	"testing"
)

func TestMapClaims_UnmarshalJSON(t *testing.T) {
	mapClaims := make(MapClaims)
	var text = `{"userId":9223372036854775807}`
	err := json.Unmarshal([]byte(text), &mapClaims)
	if err != nil {
		t.Error(err)
	}
	if numUserId, ok := mapClaims["userId"].(json.Number); !ok {
		t.Errorf("userId的类型应该是json.Number,实际:%T", mapClaims["numUserId"])
	} else {
		userId, err := numUserId.Int64()
		if err != nil {
			t.Error(err)
		}

		if userId != 9223372036854775807 {
			t.Errorf("userId的值应该相等,实际:%v", userId)
		}
	}
}
