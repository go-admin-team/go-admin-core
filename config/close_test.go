package config

import (
	"sync"
	"testing"
)

// Close is called from more than one place — a caller's defer and the watcher
// shutting down — so it has to survive being called twice at once. The previous
// implementation selected on the channel and then closed it, which is a check
// followed by an act: two callers could both find it open and both close it,
// and the second one panics.
func TestCloseIsSafeUnderConcurrency(t *testing.T) {
	for i := 0; i < 3000; i++ {
		c, err := NewConfig()
		if err != nil {
			t.Fatalf("NewConfig: %v", err)
		}

		var wg sync.WaitGroup
		start := make(chan struct{})
		for j := 0; j < 32; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				if err := c.Close(); err != nil {
					t.Errorf("Close: %v", err)
				}
			}()
		}
		close(start)
		wg.Wait()
	}
}
