package pkg

import (
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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

// The fallback is asserted directly. Driving it through get cannot see it: an
// unbounded client and a thirty-second one behave identically for every
// response that arrives, and the only case that separates them takes thirty
// seconds to reach.
func TestEffectiveTimeoutNeverYieldsAnUnboundedClient(t *testing.T) {
	for _, d := range []time.Duration{0, -time.Second} {
		if got := effectiveTimeout(d); got != getTimeout {
			t.Errorf("effectiveTimeout(%v) = %v, want %v", d, got, getTimeout)
		}
	}
	if got := effectiveTimeout(time.Second); got != time.Second {
		t.Errorf("effectiveTimeout(1s) = %v, want it left alone", got)
	}
}

// A server that accepts and never answers is what the timeout is for. Without
// one the request never returns and the caller's goroutine is gone for good.
func TestGetGivesUpOnAServerThatNeverAnswers(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
	}))
	defer func() {
		close(block)
		srv.Close()
	}()

	done := make(chan error, 1)
	go func() {
		_, err := get(srv.URL, 200*time.Millisecond)
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Error("a request to a server that never answered returned no error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the request never came back: there is no bound on it")
	}
}
