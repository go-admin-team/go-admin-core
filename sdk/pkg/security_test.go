package pkg

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// No test can tell crypto/rand from math/rand by looking at what comes out —
// both produce values that pass every distribution check worth writing here.
// The property is which source the code draws from, so it is asserted where it
// is decidable: in the imports.
func TestKeysAreDrawnFromACryptographicSource(t *testing.T) {
	// Located from this file rather than from the working directory: go test
	// runs in the package directory, but the compiled binary can be run from
	// anywhere, and a security check that quietly stops checking is worse than
	// one that was never written.
	_, self, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate this test file")
	}

	f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(filepath.Dir(self), "security.go"), nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	found := false
	for _, spec := range f.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		switch path {
		case "math/rand", "math/rand/v2":
			t.Errorf("security.go imports %s; key material needs crypto/rand", path)
		case "crypto/rand":
			found = true
		}
	}
	if !found {
		t.Error("security.go does not import crypto/rand")
	}
}

func TestGeneratedKeysHaveTheirLengthAndCharset(t *testing.T) {
	cases := []struct {
		name    string
		gen     func() string
		length  int
		charset string
	}{
		{"GenerateRandomKey20", GenerateRandomKey20, 20, symbol},
		{"GenerateRandomKey16", GenerateRandomKey16, 16, symbol},
		{"GenerateRandomKey6", GenerateRandomKey6, 6, letter},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for i := 0; i < 200; i++ {
				got := c.gen()
				if len(got) != c.length {
					t.Fatalf("length %d, want %d", len(got), c.length)
				}
				for _, r := range got {
					if !strings.ContainsRune(c.charset, r) {
						t.Fatalf("%q contains %q, which is outside the charset", got, r)
					}
				}
			}
		})
	}
}

// A generator stuck on one value, or reseeded identically per call, would pass
// the length and charset checks and fail here.
func TestGeneratedKeysDiffer(t *testing.T) {
	const runs = 500

	seen := make(map[string]struct{}, runs)
	for i := 0; i < runs; i++ {
		seen[GenerateRandomKey16()] = struct{}{}
	}
	if len(seen) != runs {
		t.Errorf("%d distinct values out of %d; the generator is repeating itself", len(seen), runs)
	}
}

// The six character key draws from 36 symbols, so collisions in 500 draws are
// expected — around a one in twenty thousand chance of a given pair matching.
// What must not happen is the value being constant.
func TestTheShortKeyIsNotConstant(t *testing.T) {
	first := GenerateRandomKey6()
	for i := 0; i < 100; i++ {
		if GenerateRandomKey6() != first {
			return
		}
	}
	t.Errorf("a hundred draws all returned %q", first)
}

func TestSetPassword(t *testing.T) {
	t.Run("the same password and salt give the same verifier", func(t *testing.T) {
		a, err := SetPassword("hunter2", "salt")
		if err != nil {
			t.Fatalf("SetPassword: %v", err)
		}
		b, err := SetPassword("hunter2", "salt")
		if err != nil {
			t.Fatalf("SetPassword: %v", err)
		}
		if a != b {
			t.Errorf("%q != %q; verification would fail for a correct password", a, b)
		}
	})

	t.Run("the salt changes the verifier", func(t *testing.T) {
		a, _ := SetPassword("hunter2", "salt-a")
		b, _ := SetPassword("hunter2", "salt-b")
		if a == b {
			t.Error("two salts produced one verifier; the salt is not reaching the derivation")
		}
	})

	t.Run("the password changes the verifier", func(t *testing.T) {
		a, _ := SetPassword("hunter2", "salt")
		b, _ := SetPassword("hunter3", "salt")
		if a == b {
			t.Error("two passwords produced one verifier")
		}
	})

	t.Run("the verifier is 32 bytes of hex", func(t *testing.T) {
		got, err := SetPassword("hunter2", "salt")
		if err != nil {
			t.Fatalf("SetPassword: %v", err)
		}
		if len(got) != 64 {
			t.Errorf("length %d, want 64", len(got))
		}
	})
}
