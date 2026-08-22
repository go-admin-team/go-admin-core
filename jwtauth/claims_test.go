package jwtauth

import (
	"encoding/json"
	"math"
	"testing"
)

func TestMapClaimsInt64ReadsEveryEncodingANumberArrivesIn(t *testing.T) {
	claims := MapClaims{
		"decoded":  json.Number("9007199254740993"),
		"float":    float64(1755000000),
		"int":      7,
		"int64":    int64(9007199254740993),
		"string":   "42",
		"garbage":  []string{"not a number"},
		"unparsed": "twelve",
	}

	for key, want := range map[string]int64{
		"decoded": 9007199254740993,
		"float":   1755000000,
		"int":     7,
		"int64":   9007199254740993,
		"string":  42,
	} {
		got, ok := claims.Int64(key)
		if !ok {
			t.Errorf("%s: not read", key)
			continue
		}
		if got != want {
			t.Errorf("%s: got %d, want %d", key, got, want)
		}
	}

	for _, key := range []string{"garbage", "unparsed", "absent"} {
		if _, ok := claims.Int64(key); ok {
			t.Errorf("%s: read as a number", key)
		}
	}
}

// A float64 that is not an integer in range is not a number this claim can
// hold. Converting one is undefined and yields MaxInt64 in practice, so 1e300
// used to read as a valid identity; NaN used to read as zero, which several
// callers treat as no user at all.
func TestMapClaimsInt64RejectsFloatsThatAreNotIdentities(t *testing.T) {
	for name, v := range map[string]float64{
		"fractional":       3.14,
		"nan":              math.NaN(),
		"positive inf":     math.Inf(1),
		"negative inf":     math.Inf(-1),
		"far above range":  1e300,
		"just above range": math.MaxInt64,
	} {
		if got, ok := (MapClaims{"identity": v}).Int64("identity"); ok {
			t.Errorf("%s (%v) read as %d", name, v, got)
		}
	}

	if got, ok := (MapClaims{"identity": float64(1755000000)}).Int64("identity"); !ok || got != 1755000000 {
		t.Errorf("an ordinary integral float64 must still read: %d, %v", got, ok)
	}
}
