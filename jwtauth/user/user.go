package user

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	jwt "github.com/go-admin-team/go-admin-core/v2/jwtauth"
	"github.com/go-admin-team/go-admin-core/v2/sdk/pkg"
)

// A claim arrives as whatever encoding/json produced. A number is a float64 by
// default and a json.Number when the parser was given UseNumber; a caller that
// built the claims itself may have left an int. Reading one with a bare type
// assertion panics on every case but the one it names — inside a request
// handler, for a token an attacker controls the shape of.
//
// float64 is also the reason an identity above 2^53 cannot survive the trip:
// it has no exact representation. json.Number keeps the digits, so it is
// preferred where present.
func claimInt(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	case float64:
		return int64(n), true
	case int:
		return int64(n), true
	case int64:
		return n, true
	case string:
		i, err := strconv.ParseInt(n, 10, 64)
		return i, err == nil
	default:
		return 0, false
	}
}

func claimString(v interface{}) (string, bool) {
	switch s := v.(type) {
	case string:
		return s, true
	case json.Number:
		return s.String(), true
	default:
		return "", false
	}
}

func ExtractClaims(c *gin.Context) jwt.MapClaims {
	claims, exists := c.Get(jwt.JwtPayloadKey)
	if !exists {
		return make(jwt.MapClaims)
	}

	return claims.(jwt.MapClaims)
}

func Get(c *gin.Context, key string) interface{} {
	data := ExtractClaims(c)
	if data[key] != nil {
		return data[key]
	}
	fmt.Println(pkg.GetCurrentTimeStr() + " [WARING] " + c.Request.Method + " " + c.Request.URL.Path + " Get 缺少 " + key)
	return nil
}

func GetUserId(c *gin.Context) int {
	data := ExtractClaims(c)
	if data["identity"] != nil {
		if id, ok := claimInt(data["identity"]); ok {
			return int(id)
		}
		fmt.Println(pkg.GetCurrentTimeStr() + " [WARING] " + c.Request.Method + " " + c.Request.URL.Path + " GetUserId: identity is not a number")
		return 0
	}
	fmt.Println(pkg.GetCurrentTimeStr() + " [WARING] " + c.Request.Method + " " + c.Request.URL.Path + " GetUserId 缺少 identity")
	return 0
}

func GetUserIdStr(c *gin.Context) string {
	data := ExtractClaims(c)
	if data["identity"] != nil {
		if id, ok := claimInt(data["identity"]); ok {
			return pkg.Int64ToString(id)
		}
		fmt.Println(pkg.GetCurrentTimeStr() + " [WARING] " + c.Request.Method + " " + c.Request.URL.Path + " GetUserIdStr: identity is not a number")
		return ""
	}
	fmt.Println(pkg.GetCurrentTimeStr() + " [WARING] " + c.Request.Method + " " + c.Request.URL.Path + " GetUserIdStr 缺少 identity")
	return ""
}

func GetUserName(c *gin.Context) string {
	data := ExtractClaims(c)
	if data["nice"] != nil {
		if s, ok := claimString(data["nice"]); ok {
			return s
		}
		return ""
	}
	fmt.Println(pkg.GetCurrentTimeStr() + " [WARING] " + c.Request.Method + " " + c.Request.URL.Path + " GetUserName 缺少 nice")
	return ""
}

func GetRoleName(c *gin.Context) string {
	data := ExtractClaims(c)
	if data["rolekey"] != nil {
		if s, ok := claimString(data["rolekey"]); ok {
			return s
		}
		return ""
	}
	fmt.Println(pkg.GetCurrentTimeStr() + " [WARING] " + c.Request.Method + " " + c.Request.URL.Path + " GetRoleName 缺少 rolekey")
	return ""
}

func GetRoleId(c *gin.Context) int {
	data := ExtractClaims(c)
	if data["roleid"] != nil {
		id, ok := claimInt(data["roleid"])
		if !ok {
			return 0
		}
		i := int(id)
		return i
	}
	fmt.Println(pkg.GetCurrentTimeStr() + " [WARING] " + c.Request.Method + " " + c.Request.URL.Path + " GetRoleId 缺少 roleid")
	return 0
}

func GetDeptId(c *gin.Context) int {
	data := ExtractClaims(c)
	if data["deptid"] != nil {
		id, ok := claimInt(data["deptid"])
		if !ok {
			return 0
		}
		i := int(id)
		return i
	}
	fmt.Println(pkg.GetCurrentTimeStr() + " [WARING] " + c.Request.Method + " " + c.Request.URL.Path + " GetDeptId 缺少 deptid")
	return 0
}

func GetDeptName(c *gin.Context) string {
	data := ExtractClaims(c)
	if data["deptkey"] != nil {
		if s, ok := claimString(data["deptkey"]); ok {
			return s
		}
		return ""
	}
	fmt.Println(pkg.GetCurrentTimeStr() + " [WARING] " + c.Request.Method + " " + c.Request.URL.Path + " GetDeptName 缺少 deptkey")
	return ""
}
