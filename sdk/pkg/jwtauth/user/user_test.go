package user

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	jwt "github.com/go-admin-team/go-admin-core/sdk/pkg/jwtauth"
	"testing"
)

func TestGetUserId(t *testing.T) {
	mapClaims := make(jwt.MapClaims)
	var text = `{"identity":9223372036854775807}`
	err := json.Unmarshal([]byte(text), &mapClaims)
	if err != nil {
		t.Error(err)
	}

	c := &gin.Context{}

	c.Set(jwt.JwtPayloadKey, mapClaims)
	userIdInt64 := GetUserIdInt64(c)
	if userIdInt64 != 9223372036854775807 {
		t.Errorf("got %v, want %v", userIdInt64, 9223372036854775807)
	}
}

func TestGetUserIdStr(t *testing.T) {
	mapClaims := make(jwt.MapClaims)
	var text = `{"identity":9223372036854775807}`
	err := json.Unmarshal([]byte(text), &mapClaims)
	if err != nil {
		t.Error(err)
	}

	c := &gin.Context{}

	c.Set(jwt.JwtPayloadKey, mapClaims)
	userIdInt64 := GetUserIdStr(c)
	if userIdInt64 != "9223372036854775807" {
		t.Errorf("got %v, want %v", userIdInt64, 9223372036854775807)
	}
}
