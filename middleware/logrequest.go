package middleware

import (
	"github.com/gabibotos/go-srv/log"
	"github.com/google/uuid"
	"golang.org/x/net/context"
	"net/http"
	"time"

	"github.com/felixge/httpsnoop"
)

func generateRequestID() string {
	// Generate a unique request ID using UUID.
	return uuid.New().String()
}

func LogRequests(lg log.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			start := time.Now()

			status := http.StatusOK
			rw = httpsnoop.Wrap(rw, httpsnoop.Hooks{
				WriteHeader: func(next httpsnoop.WriteHeaderFunc) httpsnoop.WriteHeaderFunc {
					return func(code int) {
						status = code
						next(code)
					}
				},
			})

			// Generate a unique request ID for this request.
			requestID := generateRequestID()

			// Attach the request ID to the context.
			ctx := context.WithValue(r.Context(), "requestID", requestID)

			defer func() {
				lg.Printf(
					"http request host=%s proto=%s method=%s path=%s status=%d took=%s requestID=%s",
					r.RemoteAddr,
					r.Proto,
					r.Method,
					r.RequestURI,
					status,
					time.Since(start).String(),
					requestID,
				)
			}()

			// Pass the request with the updated context to the next handler.
			next.ServeHTTP(rw, r.WithContext(ctx))
		})
	}
}
