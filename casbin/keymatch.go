package mycasbin

import (
	"errors"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
)

// The matcher calls keyMatch2(r.obj, p.obj) once per policy whose subject
// matches the request, and casbin's implementation compiles a regexp on every
// one of those calls: util.KeyMatch2 delegates to util.RegexMatch, which is
// regexp.MatchString. The package does keep a compile cache, but only KeyGet2,
// KeyGet3 and KeyMatch4 consult it.
//
// The cost is linear in the permissions a role holds - about 2.1us and 98
// allocations per policy, so a role with 200 of them spends ~420us inside a
// single Enforce. Caching the compiled form removes that work; the pattern a
// route compiles to never changes, so no result changes with it.
//
// The upstream fix is small - RegexMatch calling mustCompileOrGet instead of
// regexp.MatchString - so if casbin wires its own cache to KeyMatch2, this
// file and the matcher rename below can go away.
//
// keyMatchCacheLimit bounds the cache. Patterns come from the policy side of
// the matcher, so the distinct set is bounded by the API surface rather than
// by traffic, and the limit is only a backstop against a model that puts a
// request value there - past it, matching still works, uncached.
const keyMatchCacheLimit = 4096

// keyMatch2CachedName is the name the matcher in mycasbin.go calls. It cannot
// be "keyMatch2": FunctionMap.AddFunction stores with LoadOrStore, so adding a
// name casbin already defines is silently a no-op, and the builtin would keep
// running with nothing to show that the replacement was ignored.
const keyMatch2CachedName = "keyMatch2Cached"

var (
	// keyMatch2Param and the replacement below are casbin's, kept identical so
	// the compiled expression is the one its KeyMatch2 would have built.
	keyMatch2Param = regexp.MustCompile(`:[^/]+`)

	keyMatch2Cache     sync.Map // pattern -> *regexp.Regexp
	keyMatch2CacheSize atomic.Int64
)

var errKeyMatch2Args = errors.New("keyMatch2: expected 2 string arguments")

// KeyMatch2 reports whether path matches the route pattern, e.g.
// "/api/v1/dept/7" against "/api/v1/dept/:id".
//
// It answers exactly what casbin's util.KeyMatch2 answers, without
// recompiling the pattern on every call. Callers that match a request path
// against a fixed list of route patterns - an authorization allowlist, say -
// pay a regexp compilation per entry per request otherwise.
//
// The error reports a pattern that will not compile; util.KeyMatch2 panics on
// those.
func KeyMatch2(path, pattern string) (bool, error) {
	re, err := compileKeyMatch2(pattern)
	if err != nil {
		return false, err
	}
	return re.MatchString(path), nil
}

func compileKeyMatch2(pattern string) (*regexp.Regexp, error) {
	if v, ok := keyMatch2Cache.Load(pattern); ok {
		return v.(*regexp.Regexp), nil
	}

	expr := strings.Replace(pattern, "/*", "/.*", -1)
	expr = keyMatch2Param.ReplaceAllString(expr, "$1[^/]+$2")
	re, err := regexp.Compile("^" + expr + "$")
	if err != nil {
		// util.KeyMatch2 panics here. Enforce recovers panics into errors, so
		// the outcome there is the same either way - but a direct caller, like
		// an authorization allowlist walked in a middleware, gets an error it
		// can log instead of a panic to recover from.
		return nil, err
	}

	if keyMatch2CacheSize.Load() < keyMatchCacheLimit {
		if _, loaded := keyMatch2Cache.LoadOrStore(pattern, re); !loaded {
			keyMatch2CacheSize.Add(1)
		}
	}
	return re, nil
}

// keyMatch2Func adapts keyMatch2 to the signature casbin's evaluator expects.
func keyMatch2Func(args ...interface{}) (interface{}, error) {
	if len(args) != 2 {
		return false, errKeyMatch2Args
	}
	path, ok := args[0].(string)
	if !ok {
		return false, errKeyMatch2Args
	}
	pattern, ok := args[1].(string)
	if !ok {
		return false, errKeyMatch2Args
	}
	return KeyMatch2(path, pattern)
}
