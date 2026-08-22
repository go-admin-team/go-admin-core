package jwtauth

import (
	"encoding/json"
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
