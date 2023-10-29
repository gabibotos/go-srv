package srv

import (
	"crypto/tls"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/a-h/hsts"
	"github.com/gabibotos/go-srv/srv/schema"
	flag "github.com/spf13/pflag"
	"golang.org/x/net/context"
	"golang.org/x/sync/errgroup"
)

var defaultSchemes []string

func init() {
	defaultSchemes = []string{
		schema.SchemeHTTP,
	}
}

var (
	enabledListeners []string
	cleanupTimout    time.Duration
	maxHeaderSize    ByteSize

	DefaultHTTPFlags schema.HTTPFlg
	DefaultTLSFlags  schema.TLSFlg
)

func init() {
	maxHeaderSize = *NewByteSize(1000000)
	DefaultHTTPFlags.Host = stringEnvOverride(DefaultHTTPFlags.Host, "localhost", "HOST")
	DefaultHTTPFlags.Port = intEnvOverride(DefaultHTTPFlags.Port, 8080, "PORT")
	DefaultTLSFlags.Host = stringEnvOverride(DefaultTLSFlags.Host, "", "TLS_HOST")
	DefaultTLSFlags.Port = intEnvOverride(DefaultTLSFlags.Port, 8443, "TLS_PORT")
	DefaultTLSFlags.Cert = stringEnvOverride(DefaultTLSFlags.Cert, "", "TLS_CERTIFICATE")
	DefaultTLSFlags.CertKey = stringEnvOverride(DefaultTLSFlags.CertKey, "", "TLS_PRIVATE_KEY")
	DefaultTLSFlags.CACert = stringEnvOverride(DefaultTLSFlags.CACert, "", "TLS_CA_CERTIFICATE")
}

type (
	defaultServer struct {
		opts   *options
		CleanupTimeout   time.Duration
		MaxHeaderSize    ByteSize

		listening    bool
		shutdown     chan struct{}
		shuttingDown int32
		interrupted  bool
		interrupt    chan os.Signal
	}

	hstsConfig struct {
		MaxAge      time.Duration
		SendPreload bool
	}

	compositeHook struct {
		hooks []schema.Hook
	}
)

func New(opts ...Option) Server {
	s := &defaultServer{
		opts: newDefaultWithOptions(opts...),
		CleanupTimeout:   cleanupTimout,
		MaxHeaderSize:    maxHeaderSize,
		shutdown:         make(chan struct{}),
		interrupt:        make(chan os.Signal, 1),
	}

	if s.opts.hsts != nil {
		h := hsts.NewHandler(s.opts.handler)
		h.MaxAge = s.opts.hsts.MaxAge
		h.SendPreloadDirective = s.opts.hsts.SendPreload
		s.opts.handler = h
	}
	return s
}

func (s *defaultServer) Serve() error {
	if !s.listening {
		if err := s.Listen(); err != nil {
			return err
		}
	}

	once := new(sync.Once)
	signalNotify(s.interrupt)
	go handleInterrupt(once, s)

	servers := []*http.Server{}

	serveGroup, _ := errgroup.WithContext(context.Background())
	serveGroup.Go(func() error {
		return s.handleShutdown(&servers)
	})

	// Start regular listeners
	for _, server := range s.opts.listeners {
		if s.hasScheme(server.Scheme()) {
			sc := schema.ServerConfig{
				Callbacks:      s.opts.callbacks,
				CleanupTimeout: s.CleanupTimeout,
				MaxHeaderSize:  int(s.MaxHeaderSize.Get()),
				Handler:        s.opts.handler,
				Logger:         s.opts.logger,
			}
			if hs, err := server.Serve(sc, serveGroup); err == nil {
				servers = append(servers, hs)
			} else {
				return err
			}
		}
	}

	// Start system listeners
	for _, server := range s.opts.systemListeners {
		sc := schema.ServerConfig{
			CleanupTimeout: s.CleanupTimeout,
			MaxHeaderSize:  int(s.MaxHeaderSize.Get()),
			Handler:        s.opts.systemHandler,
			Logger:         s.opts.logger,
		}
		if hs, err := server.Serve(sc, serveGroup); err == nil {
			servers = append(servers, hs)
		} else {
			return err
		}
	}

	if err := serveGroup.Wait(); err != nil {
		return err
	}
	return nil
}

// Listen creates the listeners for the server
func (s *defaultServer) Listen() error {
	for _, server := range append(s.opts.listeners, s.opts.systemListeners...) {
		if !s.hasScheme(server.Scheme()) {
			continue
		}
		_, err := server.Listener()
		if err != nil {
			return err
		}
	}
	s.listening = true
	return nil
}

// Shutdown server and clean up resources
func (s *defaultServer) Shutdown() error {
	// ensure shutDown is performed only once
	if atomic.CompareAndSwapInt32(&s.shuttingDown, 0, 1) {
		close(s.shutdown)
	}
	return nil
}

func (s *defaultServer) handleShutdown(serversPtr *[]*http.Server) error {
	<-s.shutdown

	servers := *serversPtr

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	stGroup, stCtx := errgroup.WithContext(ctx)

	for _, srv := range servers {
		server := srv // capture the loop variable
		stGroup.Go(func() error {
			return server.Shutdown(stCtx)
		})
	}

	if err := stGroup.Wait(); err != nil {
		s.opts.logger.Fatalf("HTTP server Shutdown: %v", err)
		return err
	} else {
		if s.opts.onShutdown != nil {
			s.opts.onShutdown()
		}
	}
	return nil
}

// GetHandler returns a handler useful for testing
func (s *defaultServer) GetHandler() http.Handler {
	return s.opts.handler
}

// HTTPListener returns the http listener
func (s *defaultServer) HTTPListener() (net.Listener, error) {
	if !s.hasScheme(DefaultHTTPFlags.Scheme()) {
		return nil, nil
	}
	for _, l := range s.opts.listeners {
		if l.Scheme() == schema.SchemeHTTP {
			return l.Listener()
		}
	}
	return DefaultHTTPFlags.Listener()
}

// TLSListener returns the https listener
func (s *defaultServer) TLSListener() (net.Listener, error) {
	if !s.hasScheme(DefaultTLSFlags.Scheme()) {
		return nil, nil
	}
	for _, l := range s.opts.listeners {
		if l.Scheme() == schema.SchemeHTTPS {
			return l.Listener()
		}
	}
	return DefaultTLSFlags.Listener()
}

func handleInterrupt(once *sync.Once, s *defaultServer) {
	once.Do(func() {
		for range s.interrupt {
			if s.interrupted {
				continue
			}
			s.opts.logger.Printf("Shutting down... ")
			s.interrupted = true
			if err := s.Shutdown(); err != nil {
				s.opts.logger.Printf("error during server shutdown: %v", err)
			}
		}
	})
}

func signalNotify(interrupt chan<- os.Signal) {
	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)
}

func (s *defaultServer) hasScheme(scheme string) bool {
	schemes := s.opts.EnabledListeners
	if len(schemes) == 0 {
		schemes = defaultSchemes
	}
	enabledSchemes := make(map[string]bool)

	// Populate the map with enabled schemes
	for _, v := range schemes {
		enabledSchemes[v] = true
	}

	// Check if the given scheme is enabled
	return enabledSchemes[scheme]
}


// RegisterFlags to the specified pflag set
func RegisterFlags(fs *flag.FlagSet) {
	fs.StringSliceVar(&enabledListeners, "scheme", defaultSchemes, "the listeners to enable, this can be repeated and defaults to the schemes in the swagger spec")
	fs.DurationVar(&cleanupTimout, "cleanup-timeout", 10*time.Second, "grace period for which to wait before shutting down the server")
	fs.Var(&maxHeaderSize, "max-header-size", "controls the maximum number of bytes the server will read parsing the request header's keys and values, including the request line. It does not limit the size of the request body")

	DefaultHTTPFlags.RegisterFlags(fs)
	DefaultTLSFlags.RegisterFlags(fs)
}

func (c *compositeHook) ConfigureTLS(cfg *tls.Config) {
	for _, h := range c.hooks {
		h.ConfigureTLS(cfg)
	}
}

func (c *compositeHook) ConfigureListener(s *http.Server, scheme, addr string) {
	for _, h := range c.hooks {
		h.ConfigureListener(s, scheme, addr)
	}
}