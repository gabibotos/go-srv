package main

import (
	"github.com/gabibotos/go-srv/srv"
	"github.com/gabibotos/go-srv/srv/schema"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
	"time"
)

type (
	Option func(*options)

	options struct {
		systemOpts []srv.Option
		httpOpts []srv.Option
		isPublic  bool

		tracer  trace.Exporter
		metrics view.Exporter
	}
)

func newDefaultWithOptions(s *appsrv, opts ...Option) *options {
	o := &options{
		httpOpts: []srv.Option{
			srv.LogsWith(s.lg),
			srv.WithListeners(&schema.HTTPFlg{
				Prefix: "app",
				Host: "localhost",
				Port: 8080,
				ListenLimit: 10,
				KeepAlive: 5*time.Second,
				ReadTimeout: 3*time.Second,
				WriteTimeout: 3*time.Second,
			}),
			srv.HandlesRequestsWith(s.app),
		},
		systemOpts: []srv.Option{
			srv.WithSystemListeners(&schema.HTTPFlg{
				Prefix: "metrics",
				Host: "localhost",
				Port:   10239,
			}),
			srv.HandlesSystemWith(s.systemApp),
		},
	}

	for _, apply := range opts {
		apply(o)
	}

	return o
}

// WithHTTPOption configures the go-http server.
//noinspection GoUnusedExportedFunction
func WithHTTPOption(opts ...srv.Option) Option {
	return func(srv *options) {
		srv.httpOpts = append(srv.httpOpts, opts...)
	}
}

// WithSystemHTTPOption configures the go-http server.
//noinspection GoUnusedExportedFunction
func WithSystemHTTPOption(opts ...srv.Option) Option {
	return func(srv *options) {
		srv.systemOpts = append(srv.systemOpts, opts...)
	}
}

// IsPublic lets the server know that it's going to be the first hop
//noinspection GoUnusedExportedFunction
func IsPublic() Option {
	return func(opts *options) {
		opts.isPublic = true
	}
}

// WithTraceExprt enable opencensus trace exporting
//noinspection GoUnusedExportedFunction
func WithTraceExprt(exp trace.Exporter) Option {
	return func(opts *options) {
		opts.tracer = exp
	}
}

// WithMetricsExprt enable opencensus metrics exporter
//noinspection GoUnusedExportedFunction
func WithMetricsExprt(exp view.Exporter) Option {
	return func(opts *options) {
		opts.metrics = exp
	}
}
