package ws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// dialedClient gives back a Client wired to a real connection, because Write
// closes the socket on its way out and a nil one panics there.
func dialedClient(t *testing.T) *Client {
	t.Helper()

	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Hold the connection open; the test drives the other end.
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	t.Cleanup(srv.Close)

	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	return &Client{
		Id:         "test",
		Group:      "test",
		Context:    ctx,
		CancelFunc: cancel,
		Socket:     conn,
		Message:    make(chan []byte, 1),
	}
}

// Write takes a context as a parameter and also reads c.Context, and it used to
// depend on those being the same value: the select's break left the select
// rather than the loop, so the only thing that ended it was the check at the
// top of the loop, which watches the parameter. Cancelling the client's own
// context then spun the goroutine at full speed instead of ending it.
//
// The one call site in this module passes the same context to both, which is
// why nothing has been observed. The signature does not require it.
func TestWriteEndsWhenTheClientContextIsCancelled(t *testing.T) {
	c := dialedClient(t)

	done := make(chan struct{})
	go func() {
		defer close(done)
		// Deliberately not c.Context: the caller's context outlives the client.
		c.Write(context.Background())
	}()

	c.CancelFunc()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Write did not return after the client context was cancelled")
	}
}
