// Package jwtauth forwards to the package of the same name at the top level.
//
// Deprecated: use github.com/go-admin-team/go-admin-core/jwtauth. This package
// will be removed in v2.0.0.
//
// To migrate, change the import:
//
//	old: import "github.com/go-admin-team/go-admin-core/sdk/pkg/jwtauth"
//	new: import "github.com/go-admin-team/go-admin-core/jwtauth"
package jwtauth

import (
	"github.com/gin-gonic/gin"
	newjwtauth "github.com/go-admin-team/go-admin-core/jwtauth"
	"github.com/golang-jwt/jwt/v5"
)

// Deprecated: 使用 github.com/go-admin-team/go-admin-core/jwtauth.MapClaims 替代
type MapClaims = newjwtauth.MapClaims

// Deprecated: 使用 github.com/go-admin-team/go-admin-core/jwtauth.GinJWTMiddleware 替代
type GinJWTMiddleware = newjwtauth.GinJWTMiddleware

// Deprecated: 使用 github.com/go-admin-team/go-admin-core/jwtauth.New 替代
func New(m *GinJWTMiddleware) (*GinJWTMiddleware, error) {
	return newjwtauth.New(m)
}

// Deprecated: 使用 github.com/go-admin-team/go-admin-core/jwtauth.ExtractClaims 替代
func ExtractClaims(c *gin.Context) MapClaims {
	return newjwtauth.ExtractClaims(c)
}

// Deprecated: 使用 github.com/go-admin-team/go-admin-core/jwtauth.ExtractClaimsFromToken 替代
func ExtractClaimsFromToken(token *jwt.Token) MapClaims {
	return newjwtauth.ExtractClaimsFromToken(token)
}

// Deprecated: 使用 github.com/go-admin-team/go-admin-core/jwtauth.GetToken 替代
func GetToken(c *gin.Context) string {
	return newjwtauth.GetToken(c)
}

// The identifiers below are forwarded so that this package is a complete
// stand-in. Only some of them were, which is worse than none: the package
// compiled far enough to look like a working compatibility layer and then
// failed on whatever it had left out.
var (
	// Deprecated: use jwtauth.ErrEmptyAuthHeader.
	ErrEmptyAuthHeader = newjwtauth.ErrEmptyAuthHeader
	// Deprecated: use jwtauth.ErrEmptyCookieToken.
	ErrEmptyCookieToken = newjwtauth.ErrEmptyCookieToken
	// Deprecated: use jwtauth.ErrEmptyParamToken.
	ErrEmptyParamToken = newjwtauth.ErrEmptyParamToken
	// Deprecated: use jwtauth.ErrEmptyQueryToken.
	ErrEmptyQueryToken = newjwtauth.ErrEmptyQueryToken
	// Deprecated: use jwtauth.ErrExpiredToken.
	ErrExpiredToken = newjwtauth.ErrExpiredToken
	// Deprecated: use jwtauth.ErrFailedAuthentication.
	ErrFailedAuthentication = newjwtauth.ErrFailedAuthentication
	// Deprecated: use jwtauth.ErrFailedTokenCreation.
	ErrFailedTokenCreation = newjwtauth.ErrFailedTokenCreation
	// Deprecated: use jwtauth.ErrForbidden.
	ErrForbidden = newjwtauth.ErrForbidden
	// Deprecated: use jwtauth.ErrInvalidAuthHeader.
	ErrInvalidAuthHeader = newjwtauth.ErrInvalidAuthHeader
	// Deprecated: use jwtauth.ErrInvalidClaims.
	ErrInvalidClaims = newjwtauth.ErrInvalidClaims
	// Deprecated: use jwtauth.ErrInvalidPrivKey.
	ErrInvalidPrivKey = newjwtauth.ErrInvalidPrivKey
	// Deprecated: use jwtauth.ErrInvalidPubKey.
	ErrInvalidPubKey = newjwtauth.ErrInvalidPubKey
	// Deprecated: use jwtauth.ErrInvalidSigningAlgorithm.
	ErrInvalidSigningAlgorithm = newjwtauth.ErrInvalidSigningAlgorithm
	// Deprecated: use jwtauth.ErrInvalidVerificationode.
	ErrInvalidVerificationode = newjwtauth.ErrInvalidVerificationode
	// Deprecated: use jwtauth.ErrMissingAuthenticatorFunc.
	ErrMissingAuthenticatorFunc = newjwtauth.ErrMissingAuthenticatorFunc
	// Deprecated: use jwtauth.ErrMissingExpField.
	ErrMissingExpField = newjwtauth.ErrMissingExpField
	// Deprecated: use jwtauth.ErrMissingLoginValues.
	ErrMissingLoginValues = newjwtauth.ErrMissingLoginValues
	// Deprecated: use jwtauth.ErrMissingOrigIatField.
	ErrMissingOrigIatField = newjwtauth.ErrMissingOrigIatField
	// Deprecated: use jwtauth.ErrMissingSecretKey.
	ErrMissingSecretKey = newjwtauth.ErrMissingSecretKey
	// Deprecated: use jwtauth.ErrNoPrivKeyFile.
	ErrNoPrivKeyFile = newjwtauth.ErrNoPrivKeyFile
	// Deprecated: use jwtauth.ErrNoPubKeyFile.
	ErrNoPubKeyFile = newjwtauth.ErrNoPubKeyFile
	// Deprecated: use jwtauth.ErrWrongFormatOfExp.
	ErrWrongFormatOfExp = newjwtauth.ErrWrongFormatOfExp

	// Deprecated: use jwtauth.DataScopeKey.
	DataScopeKey = newjwtauth.DataScopeKey
	// Deprecated: use jwtauth.DeptId.
	DeptId = newjwtauth.DeptId
	// Deprecated: use jwtauth.DeptName.
	DeptName = newjwtauth.DeptName
	// Deprecated: use jwtauth.IdentityKey.
	IdentityKey = newjwtauth.IdentityKey
	// Deprecated: use jwtauth.NiceKey.
	NiceKey = newjwtauth.NiceKey
	// Deprecated: use jwtauth.RKey.
	RKey = newjwtauth.RKey
	// Deprecated: use jwtauth.RoleIdKey.
	RoleIdKey = newjwtauth.RoleIdKey
	// Deprecated: use jwtauth.RoleKey.
	RoleKey = newjwtauth.RoleKey
	// Deprecated: use jwtauth.RoleNameKey.
	RoleNameKey = newjwtauth.RoleNameKey
)

// Deprecated: use jwtauth.JwtPayloadKey.
const JwtPayloadKey = newjwtauth.JwtPayloadKey
