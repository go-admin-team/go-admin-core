package pkg

import (
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Get set the request headers before checking the error from NewRequest, so a
// url it rejects dereferenced a nil request. The caller's process ended
// instead of the call returning the error NewRequest had already produced.
func TestGetReturnsAnErrorForARejectedURL(t *testing.T) {
	got, err := Get("://not a url")
	if err == nil {
		t.Fatalf("Get returned %q and no error for a url NewRequest rejects", got)
	}
	if got != "" {
		t.Errorf("Get returned %q alongside an error", got)
	}
}

func TestGetReadsTheBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "*/*" {
			t.Errorf("Accept is %q", r.Header.Get("Accept"))
		}
		_, _ = w.Write([]byte("hello"))
	}))
	defer srv.Close()

	got, err := Get(srv.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "hello" {
		t.Errorf("Get = %q, want hello", got)
	}
}

// A truncated response arrives as a read error. It used to be discarded, so a
// half-delivered body came back as a short string with no error — which reads
// exactly like a successful short response.
func TestGetReportsATruncatedBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1024")
		_, _ = w.Write([]byte("short"))
	}))
	defer srv.Close()

	got, err := Get(srv.URL)
	if err == nil {
		t.Fatalf("Get returned %q and no error for a body that ended early", got)
	}
}

// Post marshalled its argument and threw the error away, so a value that
// cannot be encoded was sent as an empty body.
func TestPostReportsAValueItCannotEncode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("the request was sent even though the body could not be encoded")
	}))
	defer srv.Close()

	got, err := Post(srv.URL, math.Inf(1), "application/json")
	if err == nil {
		t.Fatalf("Post returned %q and no error for a value json cannot encode", got)
	}
}

func TestPostSendsAndReads(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	got, err := Post(srv.URL, map[string]string{"a": "b"}, "application/json")
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if string(got) != `{"ok":true}` {
		t.Errorf("Post = %s", got)
	}
}
