package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

func Logging(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			log := log.With(
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("remote_addr", r.RemoteAddr),
			)

			log.InfoContext(r.Context(), "request started")

			responseWriter := &loggingResponseWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(responseWriter, r)

			attrs := []any{
				slog.Int("status", responseWriter.status),
				slog.Duration("duration", time.Since(start)),
			}

			switch {
			case responseWriter.status >= http.StatusInternalServerError:
				log.ErrorContext(r.Context(), "request failed", attrs...)
			case responseWriter.status >= http.StatusBadRequest:
				log.WarnContext(r.Context(), "request failed", attrs...)
			default:
				log.InfoContext(r.Context(), "request completed", attrs...)
			}
		})
	}
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *loggingResponseWriter) WriteHeader(status int) {
	if w.status == http.StatusOK {
		w.status = status
	}

	w.ResponseWriter.WriteHeader(status)
}

func (w *loggingResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
