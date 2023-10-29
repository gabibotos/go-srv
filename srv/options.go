package srv

import (
	"log"
	"net/http"
	"os"
	"time"

	lg "github.com/gabibotos/go-srv/log"
	"github.com/gabibotos/go-srv/srv/schema"
)

type (
	Option func(*options)

	options struct {
		EnabledListeners []string

		handler      http.Handler
		systemHandler http.Handler

		callbacks schema.Hook
		logger    lg.Logger

		hsts           *hstsConfig
		onShutdown     func()
		listeners      []schema.ServerListener
		systemListeners []schema.ServerListener
	}
)

func newDefaultWithOptions(opts ...Option) *options {
	o := &options{
		EnabledListeners: enabledListeners,
		onShutdown: func() {},
		systemHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		}),
		logger: log.New(os.Stderr, "[go-http]", 0),
		// listeners: []schema.ServerListener{&DefaultHTTPFlags, &DefaultTLSFlags},

	}

	for _, apply := range opts {
		apply(o)
	}

	return o
}

// LogsWith provides a logger to the server
func LogsWith(l lg.Logger) Option {
	return func(s *options) {
		s.logger = l
	}
}

// EnablesSchemes overrides the enabled schemes
func EnablesSchemes(schemes ...string) Option {
	return func(s *options) {
		s.EnabledListeners = schemes
	}
}

// OnShutdown runs the provided functions on shutdown
func OnShutdown(handlers ...func()) Option {
	return func(s *options) {
		if len(handlers) == 0 {
			return
		}
		s.onShutdown = func() {
			for _, run := range handlers {
				run()
			}
		}
	}
}

// WithListeners replaces the default listeners with the provided listeres
func WithListeners(listener schema.ServerListener, extra ...schema.ServerListener) Option {
	all := append([]schema.ServerListener{listener}, extra...)
	return func(s *options) {
		s.listeners = all
	}
}

// WithExtaListeners appends the provided listeners to the default listeners
func WithExtraListeners(listener schema.ServerListener, extra ...schema.ServerListener) Option {
	all := append([]schema.ServerListener{listener}, extra...)
	return func(s *options) {
		s.listeners = append(s.listeners, all...)
	}
}

// WithSystemListeners configures the listeners for the system endpoint (like /healthz, /readyz, /metrics)
func WithSystemListeners(listener schema.ServerListener, extra ...schema.ServerListener) Option {
	// all := append([]schema.ServerListener{listener}, extra...)
	return func(s *options) {
		// s.systemListeners = append(s.systemListeners, all...)
		s.systemListeners = []schema.ServerListener{listener}
	}
}

// HandlesSystemWith configures the handler (maybe mux) for the system endpoint (like /healthz, /readyz, /metrics)
func HandlesSystemWith(handler http.Handler) Option {
	return func(s *options) {
		s.systemHandler = handler
	}
}

// WithSystem configures the handler and the listeners for the system endpoint (like /healthz, /readyz, /metrics)
func WithSystem(handler http.Handler, listener schema.ServerListener, extra ...schema.ServerListener) Option {
	all := append([]schema.ServerListener{listener}, extra...)
	return func(s *options) {
		s.systemListeners = append(s.systemListeners, all...)
		s.systemHandler = handler
	}
}

func EnableHSTS(maxAge time.Duration, sendPreload bool) Option {
	if maxAge == 0 {
		maxAge = time.Hour * 24 * 126 // 126 days (minimum for inclusion in the Chrome HSTS list)
	}
	return func(s *options) {
		s.hsts = &hstsConfig{
			MaxAge:      maxAge,
			SendPreload: sendPreload,
		}
	}
}

// Hooks allows for registering one or more hooks for the server to call during its lifecycle
func Hooks(hook schema.Hook, extra ...schema.Hook) Option {
	h := &compositeHook{
		hooks: append([]schema.Hook{hook}, extra...),
	}
	return func(s *options) {
		s.callbacks = h
	}
}

// HandlesRequestsWith handles the http requests to the server
func HandlesRequestsWith(h http.Handler) Option {
	return func(s *options) {
		s.handler = h
	}
}
