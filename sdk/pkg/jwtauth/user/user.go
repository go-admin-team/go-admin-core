package user

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/go-admin-team/go-admin-core/sdk/pkg"
	jwt "github.com/go-admin-team/go-admin-core/sdk/pkg/jwtauth"
)

func ExtractClaims(c *gin.Context) jwt.MapClaims {
	claims, exists := c.Get(jwt.JwtPayloadKey)
	if !exists {
		return make(jwt.MapClaims)
	}

	return claims.(jwt.MapClaims)
}

func GetUserId(c *gin.Context) int {
	data := ExtractClaims(c)
	if data["identity"] != nil {
		return int((data["identity"]).(float64))
	}
	fmt.Println(pkg.GetCurrentTimeStr() + " [WARING] " + c.Request.Method + " " + c.Request.URL.Path + " GetUserId 缺少identity")
	return 0
}

func GetUserIdStr(c *gin.Context) string {
	data := ExtractClaims(c)
	if data["identity"] != nil {
		return pkg.Int64ToString(int64((data["identity"]).(float64)))
	}
	fmt.Println(pkg.GetCurrentTimeStr() + " [WARING] " + c.Request.Method + " " + c.Request.URL.Path + " GetUserIdStr 缺少identity")
	return ""
}

func GetUserName(c *gin.Context) string {
	data := ExtractClaims(c)
	if data["nice"] != nil {
		return (data["nice"]).(string)
	}
	fmt.Println(pkg.GetCurrentTimeStr() + " [WARING] " + c.Request.Method + " " + c.Request.URL.Path + " GetUserName 缺少nice")
	return ""
}

func GetRoleName(c *gin.Context) string {
	data := ExtractClaims(c)
	if data["rolekey"] != nil {
		return (data["rolekey"]).(string)
	}
	fmt.Println(pkg.GetCurrentTimeStr() + " [WARING] " + c.Request.Method + " " + c.Request.URL.Path + " GetRoleName 缺少rolekey")
	return ""
}

func GetRoleId(c *gin.Context) int {
	data := ExtractClaims(c)
	if data["roleid"] != nil {
		i := int((data["roleid"]).(float64))
		return i
	}
	fmt.Println(pkg.GetCurrentTimeStr() + " [WARING] " + c.Request.Method + " " + c.Request.URL.Path + " GetRoleId 缺少roleid")
	return 0
}
