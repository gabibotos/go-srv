package srv

import (
	"github.com/e-dard/netbug"
	"github.com/gabibotos/go-srv/srv/schema"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"log"
	"net/http"
	"os"
	"time"
)

var adminServer = &schema.HTTPFlg{
	Prefix: "admin",
	Host: "localhost",
	Port: 12034,
	ListenLimit: 10,
	KeepAlive: 5*time.Second,
	ReadTimeout: 3*time.Second,
	WriteTimeout: 3*time.Second,
}

var appServer = &schema.HTTPFlg{
	Prefix: "app",
	Host: "localhost",
	Port: 8080,
	ListenLimit: 10,
	KeepAlive: 5*time.Second,
	ReadTimeout: 3*time.Second,
	WriteTimeout: 3*time.Second,
}

func Example() {
	adminHandler := http.NewServeMux()
	netbug.RegisterHandler("/debug/", adminHandler) // trailing slash required in this call
	adminHandler.Handle("/metrics", promhttp.Handler())
	adminHandler.HandleFunc("/healthz", healthzEndpoint)
	adminHandler.HandleFunc("/readyz", readyzEndpoint)
	adminHandler.Handle("/", http.NotFoundHandler())

	apiHandler := http.NewServeMux()
	apiHandler.HandleFunc("/test1", handleTest1)
	apiHandler.HandleFunc("/test2", handleTest2)

	ll1 := log.New(os.Stderr, "[go-http]", 0)

	server := New(
		LogsWith(ll1),
		HandlesRequestsWith(apiHandler),
		WithListeners(appServer),
		WithSystem(adminHandler, adminServer),
		OnShutdown(func() {
			ll1.Println("I'm done")
		}),
	)

	if err := server.Listen(); err != nil {
		ll1.Fatal("", zap.Error(err))
	}

	if err := server.Serve(); err != nil {
		ll1.Fatal("", zap.Error(err))
	}
}

func healthzEndpoint(rw http.ResponseWriter, r *http.Request) {
	rw.Write([]byte("OK"))
}

func readyzEndpoint(rw http.ResponseWriter, r *http.Request) {
	rw.Write([]byte("OK"))
}

func handleTest1(rw http.ResponseWriter, r *http.Request) {
	rw.Write([]byte("test1"))
}

func handleTest2(rw http.ResponseWriter, r *http.Request) {
	rw.Write([]byte("test2"))
}
