package app

import (
	"fmt"
	"strconv"
	"strings"
)

// Compare compares two semantic version strings a and b, returning -1, 0,
// or 1 depending on whether a is less than, equal to, or greater than b.
// It is the one place this module compares application versions - a host's
// installer (deciding whether an install is an upgrade, a same-version
// no-op, or a rejected downgrade) and `migrate status` are both expected to
// call this rather than each writing their own comparison.
//
// Only MAJOR.MINOR.PATCH is accepted: three dot-separated, non-negative
// decimal integers, none with a leading zero. Pre-release and
// build-metadata suffixes ("-rc1", "+build5") are not parsed, and neither is
// a negative component ("-1.0.0") - either one is rejected, with an error
// that names which of the two it was, rather than guessed at. An
// application's version is written by the application's own author;
// ordering a guess against a real version the way go-version-style
// permissive parsers do would risk installing the wrong one with no error
// to show for it, and telling an operator the wrong one of these two
// reasons would send them looking for a suffix that was never there.
func Compare(a, b string) (int, error) {
	va, err := parseVersion(a)
	if err != nil {
		return 0, err
	}
	vb, err := parseVersion(b)
	if err != nil {
		return 0, err
	}
	for i := range va {
		if va[i] != vb[i] {
			if va[i] < vb[i] {
				return -1, nil
			}
			return 1, nil
		}
	}
	return 0, nil
}

// parseVersion accepts exactly MAJOR.MINOR.PATCH: three dot-separated,
// non-negative decimal integers, none with a leading zero unless the
// component is exactly "0". Anything else is an error rather than a
// best-effort guess; parseComponent reports which of the specific reasons
// applies to whichever component is at fault.
func parseVersion(v string) ([3]int, error) {
	var out [3]int
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return out, fmt.Errorf("app: version %q is not MAJOR.MINOR.PATCH", v)
	}
	for i, p := range parts {
		n, err := parseComponent(v, p)
		if err != nil {
			return out, err
		}
		out[i] = n
	}
	return out, nil
}

// parseComponent parses one dot-separated component p of version v (v is
// only kept to quote in an error), distinguishing two reasons Compare does
// not parse it that must not share one error message:
//
//   - a '+' anywhere in p, or a '-' anywhere in p other than as its very
//     first character, means p carries build metadata or a pre-release
//     identifier ("0-rc1", "0+build5") - the version is not bare
//     MAJOR.MINOR.PATCH;
//   - a '-' as the first character of p is a negative number ("-1"), which
//     strconv.Atoi parses without complaint on its own - nothing else here
//     inspects sign, so this needs its own check, and its own, accurate
//     error. Reporting this case with the pre-release/build-metadata
//     message above would be actively wrong: a caller who wrote "-1.0.0"
//     and is told about a suffix is being pointed at something that is not
//     in their version string.
//
// Everything else - an empty component, a leading zero, a non-numeric
// component - falls through to the generic "not MAJOR.MINOR.PATCH" error.
func parseComponent(v, p string) (int, error) {
	if p == "" {
		return 0, fmt.Errorf("app: version %q is not MAJOR.MINOR.PATCH", v)
	}
	if strings.Contains(p, "+") || strings.Contains(p[1:], "-") {
		return 0, fmt.Errorf("app: version %q carries a pre-release or build-metadata suffix, which Compare does not parse", v)
	}
	if strings.HasPrefix(p, "-") {
		return 0, fmt.Errorf("app: version %q has a negative component, which is not a valid MAJOR.MINOR.PATCH version", v)
	}
	if len(p) > 1 && p[0] == '0' {
		return 0, fmt.Errorf("app: version %q is not MAJOR.MINOR.PATCH", v)
	}
	n, err := strconv.Atoi(p)
	if err != nil {
		return 0, fmt.Errorf("app: version %q is not MAJOR.MINOR.PATCH", v)
	}
	return n, nil
}
