package file

import (
	"context"

	"github.com/go-admin-team/go-admin-core/v2/config/source"
)

type filePathKey struct{}

// WithPath sets the path to file
func WithPath(filePath string) source.Option {
	return func(o *source.Options) {
		if o.Context == nil {
			o.Context = context.Background()
		}
		o.Context = context.WithValue(o.Context, filePathKey{}, filePath)
	}
}
