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
	// This check's one load-bearing job is rejecting a negative component:
	// "-1.0.0" or "1.-2.0" would otherwise parse cleanly, because
	// strconv.Atoi("-1") is a perfectly valid integer, and nothing below
	// this check inspects sign. Everything else this check also happens to
	// catch - "1.0.0-rc1", "1.0.0+build5" - is caught a second time anyway,
	// by the loop below failing to strconv.Atoi a component like "0-rc1" or
	// "0+build5". Do not delete this thinking it is redundant with that
	// loop: a counterproof that only tries pre-release/build-metadata
	// suffixes stays green with this check removed, and only a negative
	// component turns it red - see TestCompareRejectsMalformedVersions's
	// "-1.0.0"/"1.-2.0"/"1.0.-3" cases.
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
