package config

import (
	"github.com/ht-qukuai-yjingy/go-admin-core/config/loader"
	"github.com/ht-qukuai-yjingy/go-admin-core/config/reader"
	"github.com/ht-qukuai-yjingy/go-admin-core/config/source"
)

// WithLoader sets the loader for manager config
func WithLoader(l loader.Loader) Option {
	return func(o *Options) {
		o.Loader = l
	}
}

// WithSource appends a source to list of sources
func WithSource(s source.Source) Option {
	return func(o *Options) {
		o.Source = append(o.Source, s)
	}
}

// WithReader sets the config reader
func WithReader(r reader.Reader) Option {
	return func(o *Options) {
		o.Reader = r
	}
}

// WithEntity sets the config Entity
func WithEntity(e Entity) Option {
	return func(o *Options) {
		o.Entity = e
	}
}
