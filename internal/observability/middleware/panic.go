package middleware

import (
	"log/slog"
	"net/http"
)

func PanicRecoveryHTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				ctx := r.Context()

				slog.ErrorContext(ctx, "panic recovered",
					slog.String("event", "app.panic"),
					slog.Any("error", rec),
				)

				w.WriteHeader(http.StatusInternalServerError)

				// Re-panic
				panic(rec)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
