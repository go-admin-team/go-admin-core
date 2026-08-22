package jwtauth

import (
	"encoding/json"
	"math"
	"strconv"
)

// Int64 reads a numeric claim without asserting how it was encoded.
//
// A claim arrives as whatever the JSON decoder produced. A number is a float64
// by default and a json.Number when the parser was given WithJSONNumber; a
// caller that built the claims itself may have left an int. A bare type
// assertion panics on every case but the one it names, over a token whose shape
// a caller can influence.
//
// float64 is also why an identity above 2^53 cannot survive the trip: it has no
// exact representation at that size, which is where snowflake ids live.
// json.Number keeps the digits.
func (m MapClaims) Int64(key string) (int64, bool) {
	switch n := m[key].(type) {
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	case float64:
		// Converting a float64 that does not fit an int64 is undefined, and in
		// practice yields MaxInt64 - so 1e300 and +Inf both read as a valid
		// identity. A fractional value would be silently truncated, and NaN
		// reads as zero, which several callers treat as "no user". None of
		// those is a number this claim can hold.
		if n != math.Trunc(n) || n < math.MinInt64 || n >= math.MaxInt64 {
			return 0, false
		}
		return int64(n), true
	case int:
		return int64(n), true
	case int64:
		return n, true
	case string:
		i, err := strconv.ParseInt(n, 10, 64)
		return i, err == nil
	default:
		return 0, false
	}
}
