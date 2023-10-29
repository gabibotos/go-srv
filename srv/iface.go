package srv

import (
	"net"
	"net/http"
)

// Server is the interface a server implements
type Server interface {
	GetHandler() http.Handler

	TLSListener() (net.Listener, error)
	HTTPListener() (net.Listener, error)

	Listen() error
	Serve() error
	Shutdown() error
}
