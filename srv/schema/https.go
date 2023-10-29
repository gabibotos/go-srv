package schema

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	flag "github.com/spf13/pflag"
	"golang.org/x/net/netutil"
	"golang.org/x/sync/errgroup"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

type TLSFlg struct {
	HTTPFlg
	Cert    string
	CertKey string
	CACert  string
}

func (t *TLSFlg) RegisterFlags(fs *flag.FlagSet) {
	prefix := t.Prefix

	fs.StringVar(&t.Host, prefixer(prefix, "tls-host"), t.Host, "the IP to listen on")
	fs.IntVar(&t.Port, prefixer(prefix, "tls-port"), t.Port, "the port to listen on for secure connections, defaults to a random value")
	fs.StringVar(&t.Cert, prefixer(prefix, "tls-certificate"), t.Cert, "the certificate to use for secure connections")
	fs.StringVar(&t.CertKey, prefixer(prefix, "tls-key"), t.CertKey, "the private key to use for secure connections")
	fs.StringVar(&t.CACert, prefixer(prefix, "tls-ca"), t.CACert, "the certificate authority file to be used with mutual TLS auth")
	fs.IntVar(&t.ListenLimit, prefixer(prefix, "tls-listen-limit"), 0, "limit the number of outstanding requests")
	fs.DurationVar(&t.KeepAlive, prefixer(prefix, "tls-keep-alive"), 3*time.Minute, "sets the TCP keep-alive timeouts on accepted connections. It prunes dead TCP connections (e.g., closing laptop mid-download)")
	fs.DurationVar(&t.ReadTimeout, prefixer(prefix, "tls-read-timeout"), 30*time.Second, "maximum duration before timing out read of the request")
	fs.DurationVar(&t.WriteTimeout, prefixer(prefix, "tls-write-timeout"), 30*time.Second, "maximum duration before timing out write of the response")
}

func (t *TLSFlg) ApplyDefaults(values *HTTPFlg) {
	if values == nil {
		return
	}
	// Use http host if https host wasn't defined
	if t.Host == "" {
		t.Host = values.Host
	}
	// Use http listen limit if https listen limit wasn't defined
	if t.ListenLimit == 0 {
		t.ListenLimit = values.ListenLimit
	}
	// Use http tcp keep alive if https tcp keep alive wasn't defined
	if int64(t.KeepAlive) == 0 {
		t.KeepAlive = values.KeepAlive
	}
	// Use http read timeout if https read timeout wasn't defined
	if int64(t.ReadTimeout) == 0 {
		t.ReadTimeout = values.ReadTimeout
	}
	// Use http write timeout if https write timeout wasn't defined
	if int64(t.WriteTimeout) == 0 {
		t.WriteTimeout = values.WriteTimeout
	}
}

func (t *TLSFlg) Listener() (net.Listener, error) {
	var errMsg string
	t.listenOnce.Do(func() {
		addr := net.JoinHostPort(t.Host, strconv.Itoa(t.Port))
		l, err := net.Listen("tcp", addr)
		if err != nil {
			t.listener = nil
			errMsg = err.Error()
			return
		}

		hh, p, err := SplitHostPort(l.Addr().String())
		if err != nil {
			_ = l.Close()
			t.listener = nil
			errMsg = err.Error()
			return
		}

		t.Host = hh
		t.Port = p

		if t.ListenLimit > 0 {
			l = netutil.LimitListener(l, t.ListenLimit)
		}

		t.listener = l
	})

	if t.listener == nil {
		t.listenOnce = sync.Once{}
		return nil, errors.New(errMsg)
	}

	return t.listener, nil
}

func (t *TLSFlg) Serve(s ServerConfig, eg *errgroup.Group) (*http.Server, error) {
	listener, err := t.Listener()
	if err != nil {
		return nil, fmt.Errorf("failed to create listener: %v", err)
	}
	prefix := t.Prefix

	httpsServer := &http.Server{
		Addr:           listener.Addr().String(),
		MaxHeaderBytes: s.MaxHeaderSize,
		ReadTimeout:    t.ReadTimeout,
		WriteTimeout:   t.WriteTimeout,
		Handler:        s.Handler,
	}

	if int64(s.CleanupTimeout) > 0 {
		httpsServer.IdleTimeout = s.CleanupTimeout
	}

	if t.Handler != nil { // local values take precedence over the default
		httpsServer.Handler = t.Handler
	}

	httpsServer.TLSConfig = &tls.Config{
		PreferServerCipherSuites: true,
		CurvePreferences:         []tls.CurveID{tls.CurveP256, tls.X25519, tls.CurveP384},
		NextProtos:               []string{"h2", "http/1.1"},
		MinVersion:               tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
	}

	if t.Cert != "" && t.CertKey != "" {
		httpsServer.TLSConfig.Certificates = make([]tls.Certificate, 1)
		httpsServer.TLSConfig.Certificates[0], err = tls.LoadX509KeyPair(t.Cert, t.CertKey)
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS certificate and key: %v", err)
		}
	}

	if t.CACert != "" {
		caCert, caCertErr := os.ReadFile(t.CACert)
		if caCertErr != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %v", caCertErr)
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		httpsServer.TLSConfig.ClientCAs = caCertPool
		httpsServer.TLSConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	if s.Callbacks != nil {
		s.Callbacks.ConfigureTLS(httpsServer.TLSConfig)
	}

	if len(httpsServer.TLSConfig.Certificates) == 0 && httpsServer.TLSConfig.GetCertificate == nil {
		if t.Cert == "" {
			return nil, fmt.Errorf("the required flag %q was not specified", prefixer(prefix, "tls-certificate"))
		}
		if t.CertKey == "" {
			return nil, fmt.Errorf("the required flag %q was not specified", prefixer(prefix, "tls-key"))
		}
	}

	if s.Callbacks != nil {
		s.Callbacks.ConfigureListener(httpsServer, t.Scheme(), listener.Addr().String())
	}

	address := listener.Addr().String()
	p := t.Prefix
	if p == "" {
		p = t.Scheme()
	}
	s.Logger.Printf("Serving at %s://%s", p, address)
	tlsListener := tls.NewListener(listener, httpsServer.TLSConfig)
	eg.Go(func() error {
		if terr := httpsServer.Serve(tlsListener); terr != nil && terr != http.ErrServerClosed {
			s.Logger.Printf("error stopping %s listener: %v", p, terr)
			return terr
		}
		s.Logger.Printf("Stopped serving at %s://%s", p, address)
		return nil
	})

	return httpsServer, nil
}


func (t *TLSFlg) Scheme() string {
	return SchemeHTTPS
}

func (h *TLSFlg) String() string {
	return fmt.Sprintf("Prefix: %s,Host: %s,Port: %d,ListenLimit: %d,KeepAlive: %d,ReadTimeout: %d,WriteTimeout: %d\n",
		h.Prefix, h.Host, h.Port, h.ListenLimit, h.KeepAlive, h.ReadTimeout, h.WriteTimeout)
}
