package middleware

import (
	"net/http"
	_ "net/http/pprof"
)

func Profiler() http.Handler {
	return http.DefaultServeMux
}
