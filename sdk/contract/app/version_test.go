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
		// (strconv.Atoi("-1") succeeds; nothing about Atoi inspects sign),
		// so only parseComponent's dedicated leading-"-" check stops it
		// from being read as a valid, if unusual, version. See
		// TestCompareDistinguishesNegativeFromSuffix for why that check
		// has to report a reason different from a pre-release/build-
		// metadata suffix.
		"-1.0.0",
		"1.-2.0",
		"1.0.-3",
	} {
		if _, err := Compare(v, "1.0.0"); err == nil {
			t.Errorf("Compare(%q, \"1.0.0\") did not error", v)
		}
	}
}

// A negative component ("-1.0.0") and a pre-release/build-metadata suffix
// ("1.0.0-rc1") are both rejected, but for genuinely different reasons, and
// the error must say which one actually applied - not the same message for
// both. Reporting a suffix to a caller who wrote a negative number points
// them at something that is not in their version string.
func TestCompareDistinguishesNegativeFromSuffix(t *testing.T) {
	for _, v := range []string{"-1.0.0", "1.-2.0", "1.0.-3"} {
		_, err := Compare(v, "1.0.0")
		if err == nil {
			t.Fatalf("Compare(%q, ...) did not error", v)
		}
		if !strings.Contains(err.Error(), "negative component") {
			t.Errorf("Compare(%q, ...) error = %q, want it to name a negative component", v, err.Error())
		}
		if strings.Contains(err.Error(), "suffix") {
			t.Errorf("Compare(%q, ...) error = %q, wrongly blames a pre-release/build-metadata suffix", v, err.Error())
		}
	}

	for _, v := range []string{"1.0.0-rc1", "1.0.0+build5"} {
		_, err := Compare(v, "1.0.0")
		if err == nil {
			t.Fatalf("Compare(%q, ...) did not error", v)
		}
		if !strings.Contains(err.Error(), "suffix") {
			t.Errorf("Compare(%q, ...) error = %q, want it to name a pre-release/build-metadata suffix", v, err.Error())
		}
		if strings.Contains(err.Error(), "negative") {
			t.Errorf("Compare(%q, ...) error = %q, wrongly blames a negative component", v, err.Error())
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
