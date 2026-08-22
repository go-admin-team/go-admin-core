package user

import (
	"fmt"

	"github.com/gin-gonic/gin"

	jwt "github.com/go-admin-team/go-admin-core/v2/jwtauth"
	"github.com/go-admin-team/go-admin-core/v2/sdk/pkg"
)

// claimString reads a claim that has to be text.
//
// A number is not accepted in either encoding it can arrive in. Accepting
// json.Number and not float64 would make the reading depend on how the parser
// was configured: the same token would give a name of "42" with UseNumber on
// and no name at all with it off.
func claimString(v interface{}) (string, bool) {
	s, ok := v.(string)
	return s, ok
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
		if id, ok := data.Int64("identity"); ok {
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
		if id, ok := data.Int64("identity"); ok {
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
		id, ok := data.Int64("roleid")
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
		id, ok := data.Int64("deptid")
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
