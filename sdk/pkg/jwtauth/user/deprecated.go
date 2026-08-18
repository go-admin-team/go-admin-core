// Package user forwards to jwtauth/user.
//
// Deprecated: use github.com/go-admin-team/go-admin-core/jwtauth/user. This
// package exists so that the move does not break importers; it will be removed
// in v2.0.0.
//
// The parent package kept a forwarding shim when it moved, but this one did
// not, so the package simply vanished. That is a load error rather than a type
// error, which stops the compiler before it reports anything else — an
// importer upgrading to a current release saw one confusing message instead of
// the list of things actually left to change.
package user

import (
	"github.com/gin-gonic/gin"

	"github.com/go-admin-team/go-admin-core/jwtauth"
	newuser "github.com/go-admin-team/go-admin-core/jwtauth/user"
)

// Deprecated: use jwtauth/user.ExtractClaims.
func ExtractClaims(c *gin.Context) jwtauth.MapClaims { return newuser.ExtractClaims(c) }

// Deprecated: use jwtauth/user.Get.
func Get(c *gin.Context, key string) interface{} { return newuser.Get(c, key) }

// Deprecated: use jwtauth/user.GetUserId.
func GetUserId(c *gin.Context) int { return newuser.GetUserId(c) }

// Deprecated: use jwtauth/user.GetUserIdStr.
func GetUserIdStr(c *gin.Context) string { return newuser.GetUserIdStr(c) }

// Deprecated: use jwtauth/user.GetUserName.
func GetUserName(c *gin.Context) string { return newuser.GetUserName(c) }

// Deprecated: use jwtauth/user.GetRoleName.
func GetRoleName(c *gin.Context) string { return newuser.GetRoleName(c) }

// Deprecated: use jwtauth/user.GetRoleId.
func GetRoleId(c *gin.Context) int { return newuser.GetRoleId(c) }

// Deprecated: use jwtauth/user.GetDeptId.
func GetDeptId(c *gin.Context) int { return newuser.GetDeptId(c) }

// Deprecated: use jwtauth/user.GetDeptName.
func GetDeptName(c *gin.Context) string { return newuser.GetDeptName(c) }
