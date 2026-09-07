package app

import (
	"strings"
	"testing"
)

func TestCompareOrdersByNumericComponent(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.0", "1.0.1", -1},
		{"1.0.1", "1.0.0", 1},
		{"1.0.0", "1.1.0", -1},
		{"1.1.0", "1.0.0", 1},
		{"1.0.0", "2.0.0", -1},
		{"2.0.0", "1.0.0", 1},
		// Numeric, not lexicographic: "2" < "10" even though "2" > "1" as
		// a string prefix comparison would suggest.
		{"1.2.0", "1.10.0", -1},
		{"1.10.0", "1.2.0", 1},
		{"0.0.0", "0.0.0", 0},
	}
	for _, c := range cases {
		got, err := Compare(c.a, c.b)
		if err != nil {
			t.Errorf("Compare(%q, %q) returned error: %v", c.a, c.b, err)
			continue
		}
		if got != c.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

// A pre-release or build-metadata suffix must be rejected, not silently
// dropped and compared as if it were a bare MAJOR.MINOR.PATCH - see
// Compare's doc comment for why guessing is worse than erroring here.
func TestCompareRejectsPreReleaseAndBuildMetadata(t *testing.T) {
	for _, v := range []string{"1.0.0-rc1", "1.0.0+build5", "1.0.0-rc1+build5"} {
		if _, err := Compare(v, "1.0.0"); err == nil {
			t.Errorf("Compare(%q, \"1.0.0\") did not error", v)
		}
		if _, err := Compare("1.0.0", v); err == nil {
			t.Errorf("Compare(\"1.0.0\", %q) did not error", v)
		}
	}
}

func TestCompareRejectsMalformedVersions(t *testing.T) {
	for _, v := range []string{
		"",
		"1.0",
		"1.0.0.0",
		"1.0.a",
		"01.0.0",
		"1.00.0",
		"1.0.00",
		" 1.0.0",
		"1.0.0 ",
		"1..0",
		"v1.0.0",
		// A negative component parses as a valid integer on its own
		// (strconv.Atoi("-1") succeeds), so only the "-"/"+" guard for
		// pre-release/build-metadata suffixes stops it from being read as
		// a valid, if unusual, version.
		"-1.0.0",
		"1.-2.0",
		"1.0.-3",
	} {
		if _, err := Compare(v, "1.0.0"); err == nil {
			t.Errorf("Compare(%q, \"1.0.0\") did not error", v)
		}
	}
}

// The error must name the offending version, the same way
// migration.GetFilename's panic names the offending file - a caller
// reporting this error to an operator needs to say which string was bad.
func TestCompareErrorNamesTheOffendingVersion(t *testing.T) {
	_, err := Compare("not-a-version", "1.0.0")
	if err == nil {
		t.Fatal("Compare did not error on a malformed version")
	}
	if !strings.Contains(err.Error(), "not-a-version") {
		t.Errorf("error %q does not name the offending version", err.Error())
	}
}
