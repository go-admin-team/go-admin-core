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

// Group is written under manager.Lock when a client registers and read without
// it by every send service. A client connecting while a message is dispatched
// is a concurrent map read and write; ranging over it during a write is a
// fatal error rather than a failed test.
func TestManagerGroupIsNotReadWhileWritten(t *testing.T) {
	m := &Manager{
		Group:            make(map[string]map[string]*Client),
		Register:         make(chan *Client, 8),
		UnRegister:       make(chan *Client, 8),
		Message:          make(chan *MessageData, 8),
		GroupMessage:     make(chan *GroupMessageData, 8),
		BroadCastMessage: make(chan *BroadCastMessageData, 8),
	}

	go m.Start()
	go m.SendAllService()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 300; i++ {
			c := &Client{
				Id:      "c",
				Group:   "g",
				Message: make(chan []byte, 1),
			}
			// Drained, because a dispatcher blocked on a full client buffer
			// stops registering anyone — which is a real property, just not
			// the one under test here.
			go func() {
				for range c.Message {
				}
			}()

			m.Register <- c
			m.BroadCastMessage <- &BroadCastMessageData{Message: []byte("x")}
		}
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the manager stopped consuming")
	}
	time.Sleep(100 * time.Millisecond)
}
