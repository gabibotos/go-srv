package main

import (
	"github.com/gabibotos/go-srv/log"
	"github.com/gabibotos/go-srv/middleware"
	"github.com/gabibotos/go-srv/srv"
	"github.com/gorilla/mux"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/plugin/ochttp/propagation/b3"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
	"go.opencensus.io/zpages"

	"net/http"

	"github.com/heptiolabs/healthcheck"
)

/*

create git repo

create new repo using this appsrv
add viper, cobra and slog
create runtime.go and test it again

get more info about metrics and trace

checkout google wire, go project structure, some mock lib for tests

*/

type (
	appsrv struct {
		healthcheck.Handler
		opts   *options
		lg log.Logger

		server srv.Server
		app *mux.Router
		systemApp *mux.Router
	}
)

func New(log log.Logger, opts ...Option) Server {
	sysApp := mux.NewRouter()
	health := healthcheck.NewHandler()

	sysApp.Use(middleware.NoCache)
	sysApp.PathPrefix("/debug/pprof/").Handler(middleware.Profiler())
	sysApp.HandleFunc("/healthz", health.LiveEndpoint)
	sysApp.HandleFunc("/readyz", health.ReadyEndpoint)
	sysApp.HandleFunc("/version", VersionHandler(log, NewVersionInfo()))


	app := mux.NewRouter()
	app.Use(
		middleware.ProxyHeaders,
		middleware.Recover(log),
		middleware.LogRequests(log),
	)

	s := appsrv{
		lg: log,

		app: app,
		systemApp: sysApp,
		Handler: health,
	}

	s.opts = newDefaultWithOptions(&s, opts...)

	if s.opts.metrics != nil {
		if pe, ok := s.opts.metrics.(http.Handler); ok {
			s.systemApp.Handle("/metrics", pe)
		}
		err := view.Register(ochttp.ServerRequestCountView,
			ochttp.ServerRequestBytesView,
			ochttp.ServerResponseBytesView,
			ochttp.ServerLatencyView,
			ochttp.ServerRequestCountByMethod,
			ochttp.ServerResponseCountByStatusCode)
		if err != nil {
			panic(err)
		}
		view.RegisterExporter(s.opts.metrics)
	}

	if s.opts.tracer != nil {
		trace.RegisterExporter(s.opts.tracer)
		s.app.Use(
			func(next http.Handler) http.Handler {
				return &ochttp.Handler{
					Handler:          next,
					Propagation:      &b3.HTTPFormat{},
					IsPublicEndpoint: s.opts.isPublic,
				}
			},
			func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					route := mux.CurrentRoute(r)
					if route != nil {
						// Get the route path and pas it to OpenCensus HTTP handler
						path, err := route.GetPathTemplate()
						if err != nil {
							http.NotFound(w, r)
							return
						}
						ochttp.WithRouteTag(next, path).ServeHTTP(w, r)
					} else {
						// Handle case when no route is found (if needed)
						http.NotFound(w, r)
					}
				})
			},
		)

		muxx := http.NewServeMux()
		zpages.Handle(muxx, "/")
		sysApp.Handle("/rpcz", muxx)
		sysApp.Handle("/tracez", muxx)
		sysApp.Handle("/public/", muxx)
	}

	return &s
}

func (s *appsrv) App() *mux.Router {
	return s.app
}

// Init the application and its modules with the config.
func (s *appsrv) Init() error {

	srvOpts := s.opts.httpOpts[:]
	srvOpts = append(srvOpts, s.opts.systemOpts...) // force admin config
	srvOpts = append(srvOpts,
		srv.HandlesRequestsWith(s.app), // force handler config
	)
	s.server = srv.New(srvOpts...)
	return nil
}

// Start the application an its enabled modules
func (s *appsrv) Start() error {
	if err := s.server.Listen(); err != nil {
		return err
	}

	return s.server.Serve()
}

// Stop the application an its enabled modules
func (s *appsrv) Stop() error {
	if err := s.server.Shutdown(); err != nil {
		return err
	}
	return nil
}
