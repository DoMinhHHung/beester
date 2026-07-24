package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/DoMinhHHung/beester/api-gateway/internal/requestid"
)

type responseWriter struct {
	http.ResponseWriter

	statusCode   int
	bytesWritten int
}

func (w *responseWriter) WriteHeader(statusCode int) {
	if statusCode >= 100 && statusCode < 200 {
		w.ResponseWriter.WriteHeader(statusCode)
		return
	}

	if w.statusCode != 0 {
		return
	}

	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *responseWriter) Write(p []byte) (int, error) {
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}

	n, err := w.ResponseWriter.Write(p)
	w.bytesWritten += n

	return n, err
}

func (w *responseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func AccessLog(
	logger *slog.Logger,
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		writer := &responseWriter{
			ResponseWriter: w,
		}

		next.ServeHTTP(writer, r)

		statusCode := writer.statusCode
		if statusCode == 0 {
			statusCode = http.StatusOK
		}

		logger.LogAttrs(
			r.Context(),
			slog.LevelInfo,
			"http request completed",
			slog.String(
				"request_id",
				requestid.FromContext(r.Context()),
			),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", statusCode),
			slog.Int("bytes_written", writer.bytesWritten),
			slog.Duration("duration", time.Since(start)),
		)
	})
}
