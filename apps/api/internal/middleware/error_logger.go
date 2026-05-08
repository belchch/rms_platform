package middleware

import (
	"bytes"
	"net/http"

	"github.com/rs/zerolog/log"
)

type bodyRecorder struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (r *bodyRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *bodyRecorder) Write(b []byte) (int, error) {
	if r.status >= 500 {
		r.body.Write(b)
	}
	return r.ResponseWriter.Write(b)
}

func ErrorLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &bodyRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		if rec.status >= 500 {
			log.Error().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", rec.status).
				RawJSON("error", rec.body.Bytes()).
				Msg("server error")
		}
	})
}
