package main

import (
	"bytes"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRewrite(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "a plain import is replaced",
			in: `package p

import "github.com/go-admin-team/go-admin-core/sdk/pkg/response"

var _ = response.Error
`,
			want: `package p

import "github.com/go-admin-team/go-admin-core/response"

var _ = response.Error
`,
		},
		{
			name: "an alias survives",
			in: `package p

import jwt "github.com/go-admin-team/go-admin-core/sdk/pkg/jwtauth"

var _ = jwt.New
`,
			want: `package p

import jwt "github.com/go-admin-team/go-admin-core/jwtauth"

var _ = jwt.New
`,
		},
		{
			name: "a dot import stays a dot import",
			in: `package p

import . "github.com/go-admin-team/go-admin-core/tools/gorm/logger"

var _ = New
`,
			want: `package p

import . "github.com/go-admin-team/go-admin-core/tools/gorm/gormlog"

var _ = New
`,
		},
		{
			name: "a renamed package keeps the local name it had",
			in: `package p

import "github.com/go-admin-team/go-admin-core/tools/gorm/logger"

var _ = logger.New
`,
			want: `package p

import logger "github.com/go-admin-team/go-admin-core/tools/gorm/gormlog"

var _ = logger.New
`,
		},
		{
			name: "one line of a block, the rest untouched",
			in: `package p

import (
	"fmt"

	"github.com/go-admin-team/go-admin-core/sdk/pkg/captcha"
	"gorm.io/gorm"
)

var _ = fmt.Sprint
var _ = captcha.New
var _ = gorm.Open
`,
			want: `package p

import (
	"fmt"

	"github.com/go-admin-team/go-admin-core/captcha"
	"gorm.io/gorm"
)

var _ = fmt.Sprint
var _ = captcha.New
var _ = gorm.Open
`,
		},
		{
			name: "the path in a comment or a string is not an import",
			in: `package p

// See github.com/go-admin-team/go-admin-core/sdk/pkg/response for the old one.
const doc = "github.com/go-admin-team/go-admin-core/sdk/pkg/response"
`,
			want: `package p

// See github.com/go-admin-team/go-admin-core/sdk/pkg/response for the old one.
const doc = "github.com/go-admin-team/go-admin-core/sdk/pkg/response"
`,
		},
		{
			name: "a file with nothing to do is returned as it was",
			in: `package p

import "fmt"

var _ = fmt.Sprint
`,
			want: `package p

import "fmt"

var _ = fmt.Sprint
`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, changes, err := rewrite("x.go", []byte(c.in), false)
			if err != nil {
				t.Fatalf("rewrite: %v", err)
			}
			if string(got) != c.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, c.want)
			}
			if changed := c.in != c.want; changed != (len(changes) > 0) {
				t.Errorf("reported %d changes for a file that changed=%v", len(changes), changed)
			}
			if _, err := parser.ParseFile(token.NewFileSet(), "x.go", got, 0); err != nil {
				t.Errorf("output does not parse: %v", err)
			}
		})
	}
}

// Every path in the table has to exist in this module. A typo in a destination
// sends every consumer that runs the tool to a package that is not there, and
// the tool itself would never notice.
func TestMovesPointAtRealPackages(t *testing.T) {
	root := filepath.Join("..", "..")

	seen := map[string]bool{}
	for _, m := range moves {
		for _, p := range []string{m.from, m.to} {
			if !strings.HasPrefix(p, mod) {
				t.Errorf("%s is not in this module", p)
				continue
			}
			dir := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(p, mod)))
			info, err := os.Stat(dir)
			if err != nil || !info.IsDir() {
				t.Errorf("%s: no package at %s", p, dir)
				continue
			}
			files, err := filepath.Glob(filepath.Join(dir, "*.go"))
			if err != nil || len(files) == 0 {
				t.Errorf("%s: %s holds no Go files", p, dir)
			}
		}

		if m.from == m.to {
			t.Errorf("%s moves to itself", m.from)
		}
		if seen[m.from] {
			t.Errorf("%s appears twice", m.from)
		}
		seen[m.from] = true
	}
}

// The tool overwrites files in someone else's repository, so it has to leave
// their permission bits alone.
//
// This pins the contract rather than reproducing a failure: os.WriteFile only
// applies its mode when it creates the file, so the current implementation
// cannot fail this test — reverting the mode to a literal 0644 still passes.
// What it would catch is the write being changed to the usual atomic shape, a
// temporary file renamed over the target, which does replace the mode.
func TestWriteKeepsThePermissionBits(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "x.go")
	src := "package p\n\nimport \"" + mod + "sdk/pkg/response\"\n\nvar _ = response.Error\n"

	if err := os.WriteFile(file, []byte(src), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, _, err := run(dir, true, false, io.Discard); err != nil {
		t.Fatalf("run: %v", err)
	}

	info, err := os.Stat(file)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("mode is %v, want 0600: the tool changed the file's permissions", got)
	}

	out, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.Contains(string(out), "sdk/pkg/response") {
		t.Error("the import was not rewritten")
	}
}

// A replacement path usually sorts somewhere else in the block, so a rewrite
// that only swaps the string leaves the group unsorted. The first version of
// this tool did exactly that, in nineteen files of the one repository it was
// tried on.
func TestRewriteLeavesTheFileFormatted(t *testing.T) {
	in := `package p

import (
	"github.com/gin-gonic/gin"
	"github.com/go-admin-team/go-admin-core/sdk/api"
	"github.com/go-admin-team/go-admin-core/sdk/pkg/captcha"
)

var _ = gin.New
var _ = api.Api{}
var _ = captcha.New
`

	got, _, err := rewrite("x.go", []byte(in), false)
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	want, err := format.Source(got)
	if err != nil {
		t.Fatalf("format: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("output is not gofmt clean:\n%s", got)
	}
	if !strings.Contains(string(got), "core/captcha\"\n\t\"github.com/go-admin-team/go-admin-core/sdk/api\"") {
		t.Errorf("the replacement did not move to where it sorts:\n%s", got)
	}
}

// The other half of the rule: a file that was not gofmt clean to begin with is
// not quietly reformatted, or the migration disappears into whitespace.
func TestRewriteDoesNotFormatWhatItFound(t *testing.T) {
	in := `package p

import (
	"github.com/go-admin-team/go-admin-core/sdk/pkg/response"
)

var  _ = response.Error
`

	got, changes, err := rewrite("x.go", []byte(in), false)
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("changes = %d, want 1", len(changes))
	}
	if !strings.Contains(string(got), "var  _ = response.Error") {
		t.Errorf("the double space was reformatted away:\n%s", got)
	}
}

// Go requires the major version in the path from v2 on, so -v2 touches every
// import of this module, not only the ones that also moved.
func TestTargetWithMajor(t *testing.T) {
	const bare = "github.com/go-admin-team/go-admin-core"

	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "an ordinary package gains the version",
			in:   bare + "/sdk/runtime",
			want: bare + "/v2/sdk/runtime",
		},
		{
			name: "the module root gains it too",
			in:   bare,
			want: bare + "/v2",
		},
		{
			name: "a moved package is moved first, then carried across",
			in:   bare + "/sdk/pkg/response",
			want: bare + "/v2/response",
		},
		{
			name: "a path already on v2 is left alone",
			in:   bare + "/v2/sdk/runtime",
			want: bare + "/v2/sdk/runtime",
		},
		{
			name: "another module that starts the same way is not this one",
			in:   bare + "-extra/sdk/runtime",
			want: bare + "-extra/sdk/runtime",
		},
		{
			name: "someone else's module is untouched",
			in:   "gorm.io/gorm",
			want: "gorm.io/gorm",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, _, changed := target(c.in, true)
			if got != c.want {
				t.Errorf("target(%q) = %q, want %q", c.in, got, c.want)
			}
			if want := c.in != c.want; changed != want {
				t.Errorf("changed = %v, want %v", changed, want)
			}
		})
	}
}

// Without the flag nothing gains a version, which is what a consumer staying
// on v1 has to be able to rely on.
func TestTargetWithoutMajorLeavesTheVersionAlone(t *testing.T) {
	for _, in := range []string{
		"github.com/go-admin-team/go-admin-core/sdk/runtime",
		"github.com/go-admin-team/go-admin-core",
	} {
		got, _, changed := target(in, false)
		if changed || got != in {
			t.Errorf("target(%q, false) = %q, changed=%v; want it untouched", in, got, changed)
		}
	}

	// A deprecated path still moves, it just does not gain a version.
	got, _, changed := target("github.com/go-admin-team/go-admin-core/sdk/pkg/response", false)
	if !changed || got != "github.com/go-admin-team/go-admin-core/response" {
		t.Errorf("got %q changed=%v, want the v1 destination", got, changed)
	}
}

// The whole point of -v2 is one pass rather than two, so a file mixing moved
// and unmoved imports has to come out consistent.
func TestRewriteWithMajorAcrossOneFile(t *testing.T) {
	in := `package p

import (
	"github.com/go-admin-team/go-admin-core/sdk/pkg/response"
	"github.com/go-admin-team/go-admin-core/sdk/runtime"
	"gorm.io/gorm"
)

var _ = response.Error
var _ = runtime.Runtime
var _ = gorm.Open
`
	want := `package p

import (
	"github.com/go-admin-team/go-admin-core/v2/response"
	"github.com/go-admin-team/go-admin-core/v2/sdk/runtime"
	"gorm.io/gorm"
)

var _ = response.Error
var _ = runtime.Runtime
var _ = gorm.Open
`

	got, changes, err := rewrite("x.go", []byte(in), true)
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if string(got) != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if len(changes) != 2 {
		t.Errorf("changes = %d, want 2", len(changes))
	}
}
