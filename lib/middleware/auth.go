package middleware

import (
	"net/http"
	"os"
)

func AdminAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Admin-Access-Token")
		expectedToken := os.Getenv("ADMIN_ACCESS_TOKEN")

		if token != expectedToken {
			http.Error(w, "Unauthorized access", http.StatusUnauthorized)
			return
		}

		w.WriteHeader(http.StatusOK)
		next.ServeHTTP(w, r)
	})
}
