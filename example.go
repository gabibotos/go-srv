package main

import (
	"github.com/gabibotos/go-srv/middleware"
	"github.com/gabibotos/go-srv/srv"
	"github.com/gabibotos/go-srv/srv/schema"
	"go.uber.org/zap"
	"net/http"
	"time"
)

func Example() {
	lc := zap.NewProductionConfig()
	lc.Development = true

	zlg, err := lc.Build(zap.AddCallerSkip(1))
	if err != nil {
		panic(err)
	}

	ll := &zapLogger{zlg}

	ss := New(ll,
		WithHTTPOption(
			srv.WithListeners(&schema.HTTPFlg{
				Prefix: "app",
				Host: "localhost",
				Port: 8081,
				ListenLimit: 10,
				KeepAlive: 5*time.Second,
				ReadTimeout: 3*time.Second,
				WriteTimeout: 3*time.Second,
			}),
			srv.OnShutdown(func() {
				ll.Printf("OnShutdown - I'm Done")
			}),
		),
		WithSystemHTTPOption(
			srv.WithSystemListeners(
				&schema.HTTPFlg{
					Prefix: "metrics",
					Host: "localhost",
					Port:   10240,
				}),
		),
	)

	ss.App().HandleFunc("/test1", handleTest1)
	ss.App().HandleFunc("/test2", handleTest2)

	// Register HTTP API middleware here
	ss.App().Use(
		middleware.LogRequests(ll),
	)

	err = ss.Init()
	if err != nil {
		panic(err)
	}

	err = ss.Start()
	if err != nil {
		panic(err)
	}
}

type zapLogger struct {
	zlg *zap.Logger
}

func (z *zapLogger) Printf(format string, args ...interface{}) {
	z.zlg.Sugar().Infof(format, args...)
}
func (z *zapLogger) Fatalf(format string, args ...interface{}) {
	z.zlg.Sugar().Fatalf(format, args...)
}

func handleTest1(rw http.ResponseWriter, r *http.Request) {
	rw.Write([]byte("test1"))
}

func handleTest2(rw http.ResponseWriter, r *http.Request) {
	rw.Write([]byte("test2"))
}
