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
			got, changes, err := rewrite("x.go", []byte(c.in))
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

	if _, _, err := run(dir, true, io.Discard); err != nil {
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

	got, _, err := rewrite("x.go", []byte(in))
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

	got, changes, err := rewrite("x.go", []byte(in))
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
