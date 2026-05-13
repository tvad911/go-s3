package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"gos3/internal/handler"
	"gos3/internal/s3"
)

// Recoverer is a middleware that recovers from panics, logs the panic (and a
// backtrace), and returns an HTTP 500 (Internal Server Error) status if
// possible. Recoverer prints a request ID if one is provided.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rvr := recover(); rvr != nil {
				reqID, _ := r.Context().Value(RequestIDKey).(string)

				slog.Error("panic recovered",
					"request_id", reqID,
					"error", rvr,
					"stack", string(debug.Stack()),
				)

				// Write S3 Internal Error
				handler.WriteError(w, r, s3.ErrInternalError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
