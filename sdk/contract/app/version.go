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
// build-metadata suffixes ("-rc1", "+build5") are not parsed - a version
// carrying one is rejected rather than guessed at. An application's version
// is written by the application's own author; ordering a guess against a
// real version the way go-version-style permissive parsers do would risk
// installing the wrong one with no error to show for it.
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
// component is exactly "0". Anything else - a pre-release/build suffix, a
// missing or extra component, a non-numeric component, an empty component -
// is an error rather than a best-effort guess.
func parseVersion(v string) ([3]int, error) {
	var out [3]int
	if strings.ContainsAny(v, "-+") {
		return out, fmt.Errorf("app: version %q carries a pre-release or build-metadata suffix, which Compare does not parse", v)
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return out, fmt.Errorf("app: version %q is not MAJOR.MINOR.PATCH", v)
	}
	for i, p := range parts {
		if p == "" || (len(p) > 1 && p[0] == '0') {
			return out, fmt.Errorf("app: version %q is not MAJOR.MINOR.PATCH", v)
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return out, fmt.Errorf("app: version %q is not MAJOR.MINOR.PATCH", v)
		}
		out[i] = n
	}
	return out, nil
}
