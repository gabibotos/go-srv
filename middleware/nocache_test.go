package middleware

import (
	"github.com/gorilla/mux"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNoCache(t *testing.T) {

	rr := httptest.NewRecorder()
	s := mux.NewRouter()
	s.Use(NoCache)
	s.HandleFunc("/", func(w http.ResponseWriter, r *http.Request){
		w.Write([]byte("OK"))
	})
	r, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	s.ServeHTTP(rr, r)

	for k, v := range noCacheHeaders {
		if rr.HeaderMap[k][0] != v {
			t.Errorf("%s header not set by middleware.", k)
		}
	}
}
