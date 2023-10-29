package schema

import (
	"crypto/tls"
	"github.com/gabibotos/go-srv/log"
	"golang.org/x/sync/errgroup"
	"net"
	"net/http"
	"time"
)

type (
	ServerListener interface {
		Listener() (net.Listener, error)
		Serve(ServerConfig, *errgroup.Group) (*http.Server, error)
		Scheme() string
		String() string
	}

	// Hook allows for hooking into the lifecycle of the server
	Hook interface {
		ConfigureTLS(*tls.Config)
		ConfigureListener(*http.Server, string, string)
	}
)

type ServerConfig struct {
	MaxHeaderSize  int
	Logger         log.Logger
	Handler        http.Handler
	Callbacks      Hook
	CleanupTimeout time.Duration
}
