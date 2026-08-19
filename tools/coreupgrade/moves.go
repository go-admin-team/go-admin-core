package main

const mod = "github.com/go-admin-team/go-admin-core/"

// move is one deprecated import path and the path that replaces it. name and
// newName are the package names; they differ only where the promotion also
// renamed the package, which is the case a plain path substitution gets wrong.
type move struct {
	from    string
	to      string
	name    string
	newName string
}

// Every package this module deprecated in v1.6.0 and removes in v2.0.0. The
// replacements re-export the same identifiers, so nothing but the import line
// changes — except gormlog, which was renamed to stop colliding with the
// logger package at the root.
var moves = []move{
	{from: mod + "sdk/pkg/captcha", to: mod + "captcha", name: "captcha", newName: "captcha"},
	{from: mod + "sdk/pkg/jwtauth", to: mod + "jwtauth", name: "jwtauth", newName: "jwtauth"},
	{from: mod + "sdk/pkg/jwtauth/user", to: mod + "jwtauth/user", name: "user", newName: "user"},
	{from: mod + "sdk/pkg/response", to: mod + "response", name: "response", newName: "response"},
	{from: mod + "sdk/pkg/casbin", to: mod + "casbin", name: "casbin", newName: "casbin"},
	{from: mod + "observability/audit", to: mod + "observe/audit", name: "audit", newName: "audit"},
	{from: mod + "tools/gorm/logger", to: mod + "tools/gorm/gormlog", name: "logger", newName: "gormlog"},
}
