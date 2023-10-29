package schema

import (
	"errors"
	"fmt"
	flag "github.com/spf13/pflag"
	"golang.org/x/net/netutil"
	"golang.org/x/sync/errgroup"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type HTTPFlg struct {
	Prefix       string
	Host         string
	Port         int
	ListenLimit  int
	KeepAlive    time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	Handler      http.Handler

	listenOnce sync.Once
	listener   net.Listener
}

func (h *HTTPFlg) RegisterFlags(fs *flag.FlagSet) {
	prefix := h.Prefix

	fs.StringVar(&h.Host, prefixer(prefix, "host"), h.Host, "the IP to listen on")
	fs.IntVar(&h.Port, prefixer(prefix, "port"), h.Port, "the port to listen on for http connections, defaults to a random value")
	fs.IntVar(&h.ListenLimit, prefixer(prefix, "listen-limit"), 0, "limit the number of outstanding requests")
	fs.DurationVar(&h.KeepAlive, prefixer(prefix, "keep-alive"), 3*time.Minute, "sets the TCP keep-alive timeouts on accepted connections. It prunes dead TCP connections ( e.g. closing laptop mid-download)")
	fs.DurationVar(&h.ReadTimeout, prefixer(prefix, "read-timeout"), 30*time.Second, "maximum duration before timing out read of the request")
	fs.DurationVar(&h.WriteTimeout, prefixer(prefix, "write-timeout"), 30*time.Second, "maximum duration before timing out write of the response")
}

func (h *HTTPFlg) Listener() (net.Listener, error) {
	var errMsg string
	h.listenOnce.Do(func() {
		l, err := net.Listen("tcp", net.JoinHostPort(h.Host, strconv.Itoa(h.Port)))
		if err != nil {
			h.listener = nil
			errMsg = err.Error()
			return
		}

		hh, p, err := SplitHostPort(l.Addr().String())
		if err != nil {
			_ = l.Close()
			h.listener = nil
			errMsg = err.Error()
			return
		}

		h.Host = hh
		h.Port = p

		if h.ListenLimit > 0 {
			l = netutil.LimitListener(l, h.ListenLimit)
		}

		h.listener = l
	})

	if h.listener == nil {
		h.listenOnce = sync.Once{}
		return nil, errors.New(errMsg)
	}

	return h.listener, nil
}

func (h *HTTPFlg) Serve(s ServerConfig, eg *errgroup.Group) (*http.Server, error) {
	listener, err := h.Listener()
	if err != nil {
		return nil, err
	}

	httpSrv := &http.Server{
		MaxHeaderBytes: s.MaxHeaderSize,
		ReadTimeout:    h.ReadTimeout,
		WriteTimeout:   h.WriteTimeout,
		Handler:        s.Handler,
	}

	if int64(s.CleanupTimeout) > 0 {
		httpSrv.IdleTimeout = s.CleanupTimeout
	}

	if int64(h.KeepAlive) > 0 {
		httpSrv.SetKeepAlivesEnabled(true)
	}

	if h.Handler != nil {
		httpSrv.Handler = s.Handler
	}

	if s.Callbacks != nil {
		s.Callbacks.ConfigureListener(httpSrv, h.Scheme(), listener.Addr().String())
	}

	address := listener.Addr().String()
	p := h.Prefix
	if p == "" {
		p = h.Scheme()
	}
	s.Logger.Printf("Serving at %s://%s", p, address)
	eg.Go(func() error {
		if herr := httpSrv.Serve(listener); herr != nil && herr != http.ErrServerClosed {
			s.Logger.Printf("Error stopping %s listener: %v", p, herr)
			return herr
		}
		s.Logger.Printf("Stopped serving at %s://%s", p, address)
		return nil
	})

	return httpSrv, nil
}

func (h *HTTPFlg) Scheme() string {
	return SchemeHTTP
}

func (h *HTTPFlg) String() string {
		return fmt.Sprintf("Prefix: %s,Host: %s,Port: %d,ListenLimit: %d,KeepAlive: %d,ReadTimeout: %d,WriteTimeout: %d\n",
			h.Prefix, h.Host, h.Port, h.ListenLimit, h.KeepAlive, h.ReadTimeout, h.WriteTimeout)
}
