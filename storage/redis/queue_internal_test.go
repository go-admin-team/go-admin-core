package redis

import (
	"testing"
)

// decodeValues is tested directly because one of its branches cannot be
// reached through the public API: the client reads every stream field with
// ReadString, so a non-string value never arrives from Redis. An end-to-end
// test can only exercise the string cases.
func TestDecodeValues(t *testing.T) {
	cases := []struct {
		name string
		in   map[string]interface{}
		want map[string]interface{}
	}{
		{
			name: "json values are decoded",
			in:   map[string]interface{}{"id": `"42"`, "count": "7", "ok": "true"},
			want: map[string]interface{}{"id": "42", "count": float64(7), "ok": true},
		},
		{
			name: "a string that is not json is passed through",
			in:   map[string]interface{}{"raw": "not json at all"},
			want: map[string]interface{}{"raw": "not json at all"},
		},
		{
			name: "a value that is not a string is passed through untouched",
			in:   map[string]interface{}{"weird": 42},
			want: map[string]interface{}{"weird": 42},
		},
		{
			name: "the empty marker yields an empty payload",
			in:   map[string]interface{}{emptyField: ""},
			want: map[string]interface{}{},
		},
		{
			name: "a caller's field of the marker's name survives",
			in:   map[string]interface{}{emptyField: `"mine"`},
			want: map[string]interface{}{emptyField: "mine"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := decodeValues(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("got %#v, want %#v", got, tc.want)
			}
			for k, want := range tc.want {
				if got[k] != want {
					t.Errorf("%q: got %#v, want %#v", k, got[k], want)
				}
			}
		})
	}
}

// Every value has to be a string on the wire, and an empty payload still needs
// one field because XADD rejects an entry with none.
func TestEncodeValues(t *testing.T) {
	out, err := encodeValues(nil)
	if err != nil {
		t.Fatalf("encode an empty payload: %v", err)
	}
	if len(out) != 1 || out[emptyField] != "" {
		t.Errorf("an empty payload must carry the marker alone, got %#v", out)
	}

	out, err = encodeValues(map[string]interface{}{"id": "42"})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if out["id"] != `"42"` {
		t.Errorf("values travel as JSON, got %#v", out["id"])
	}

	if _, err := encodeValues(map[string]interface{}{"ch": make(chan int)}); err == nil {
		t.Error("a value with no JSON form must be rejected at publish time")
	}
}
