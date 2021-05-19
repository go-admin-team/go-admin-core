package memory

import (
	"github.com/yahao333/go-admin-core/config/source"
)

type watcher struct {
	Id      string
	Updates chan *source.ChangeSet
	Source  *memory
}

func (w *watcher) Next() (*source.ChangeSet, error) {
	cs := <-w.Updates
	return cs, nil
}

func (w *watcher) Stop() error {
	w.Source.Lock()
	delete(w.Source.Watchers, w.Id)
	w.Source.Unlock()
	return nil
}
