package mycasbin

import (
	"strconv"
	"strings"
	"testing"

	"github.com/casbin/casbin/v3/util"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// TestKeyMatch2AgreesWithCasbin is the whole safety argument for replacing the
// builtin: the cached form has to answer identically, including for the shapes
// nobody writes on purpose.
func TestKeyMatch2AgreesWithCasbin(t *testing.T) {
	paths := []string{
		"/api/v1/dept", "/api/v1/dept/", "/api/v1/dept/7", "/api/v1/dept/7/sub",
		"/api/v1/roleMenuTreeselect/12", "/", "", "/api/v1", "/api/v1/",
		"/api/v1/gen/preview/3", "/api/v1/public/uploadFile", "/apix/v1/dept",
		"/api/v1/dept?x=1", "/api/v1/de.pt",
	}
	patterns := []string{
		"/api/v1/dept", "/api/v1/dept/:id", "/api/v1/dept/*", "/api/v1/*",
		"/:a/:b/:c", "/", "*", "/api/v1/gen/preview/:tableId", "",
		"/api/v1/de.pt", "/api/v1/dept/:id/sub",
	}

	for _, path := range paths {
		for _, pattern := range patterns {
			want := util.KeyMatch2(path, pattern)
			got, err := KeyMatch2(path, pattern)
			if err != nil {
				t.Fatalf("KeyMatch2(%q, %q) returned an error casbin does not: %v", path, pattern, err)
			}
			if got != want {
				t.Errorf("KeyMatch2(%q, %q) = %v, casbin says %v", path, pattern, got, want)
			}
		}
	}
}

// TestKeyMatch2CacheIsBounded checks the backstop: a caller that feeds distinct
// patterns must not be able to grow the cache without limit.
//
// Filling it has a lasting effect - past the limit nothing new is cached, by
// design - so this test puts the cache back rather than leaving every later
// test in the package running against a full one.
func TestKeyMatch2CacheIsBounded(t *testing.T) {
	t.Cleanup(func() {
		keyMatch2Cache.Range(func(k, _ any) bool {
			keyMatch2Cache.Delete(k)
			return true
		})
		keyMatch2CacheSize.Store(0)
	})

	before := keyMatch2CacheSize.Load()
	for i := 0; i < keyMatchCacheLimit+2000; i++ {
		if _, err := KeyMatch2("/api/v1/x", "/api/v1/unbounded"+strconv.Itoa(i)+"/:id"); err != nil {
			t.Fatal(err)
		}
	}
	if got := keyMatch2CacheSize.Load(); got > keyMatchCacheLimit {
		t.Fatalf("cache holds %d entries, limit is %d", got, keyMatchCacheLimit)
	}
	if keyMatch2CacheSize.Load() < before {
		t.Fatal("counter went backwards")
	}
}

// TestKeyMatch2MalformedPatternDoesNotPanic pins the one deliberate difference
// from casbin, which panics on a pattern that will not compile.
func TestKeyMatch2MalformedPatternDoesNotPanic(t *testing.T) {
	if _, err := KeyMatch2("/api/v1/dept", "/api/v1/["); err == nil {
		t.Fatal("expected an error for an uncompilable pattern")
	}
}

// TestKeyMatch2ConcurrentAgrees runs the cache under the access pattern it has
// in production - many goroutines, overlapping patterns - with -race.
func TestKeyMatch2ConcurrentAgrees(t *testing.T) {
	patterns := []string{"/api/v1/dept/:id", "/api/v1/*", "/api/v1/dept", "/:a/:b"}
	done := make(chan struct{})
	for g := 0; g < 8; g++ {
		go func(g int) {
			defer func() { done <- struct{}{} }()
			for i := 0; i < 2000; i++ {
				p := patterns[i%len(patterns)]
				got, err := KeyMatch2("/api/v1/dept/9", p)
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				if want := util.KeyMatch2("/api/v1/dept/9", p); got != want {
					t.Errorf("KeyMatch2(_, %q) = %v, want %v", p, got, want)
					return
				}
			}
		}(g)
	}
	for g := 0; g < 8; g++ {
		<-done
	}
}

// TestSetupWiresTheCachedMatcher is the regression test for the trap that made
// the first attempt a no-op: casbin's FunctionMap.AddFunction stores with
// LoadOrStore, so registering a name it already defines does nothing and the
// builtin keeps running. Nothing reports that. If the matcher and the
// registered name ever drift apart, Enforce fails to resolve the function and
// this test catches it - a path pattern is the case that needs it.
func TestSetupWiresTheCachedMatcher(t *testing.T) {
	if !strings.Contains(text, keyMatch2CachedName+"(r.obj, p.obj)") {
		t.Fatalf("the matcher does not call %s:\n%s", keyMatch2CachedName, text)
	}

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	ReloadInterval = 0
	e := Setup(db, "keymatch.example")

	if err := db.Exec(
		`INSERT INTO casbin_rule (ptype, v0, v1, v2) VALUES (?, ?, ?, ?)`,
		"p", "keymatch-role", "/api/v1/dept/:id", "GET",
	).Error; err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := e.LoadPolicy(); err != nil {
		t.Fatalf("LoadPolicy: %v", err)
	}

	cases := []struct {
		path string
		want bool
	}{
		{"/api/v1/dept/7", true},
		{"/api/v1/dept/7/sub", false},
		{"/api/v1/dept", false},
	}
	for _, c := range cases {
		got, err := e.Enforce("keymatch-role", c.path, "GET")
		if err != nil {
			t.Fatalf("Enforce(%q): %v", c.path, err)
		}
		if got != c.want {
			t.Errorf("Enforce(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}
