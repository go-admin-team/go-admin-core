package queue_test

import (
	"testing"

	"github.com/go-admin-team/go-admin-core/v2/storage"
	"github.com/go-admin-team/go-admin-core/v2/storage/cachetest"
	"github.com/go-admin-team/go-admin-core/v2/storage/queue"
)

func TestMemQueueConformance(t *testing.T) {
	cachetest.RunQueue(t, func(t *testing.T) storage.Queue {
		return queue.NewMemQueue(64)
	})
}
