package handlers

import (
	"context"
	"crypto/subtle"
	"net/http"

	"dynamics-dashboard/security"
)

type ctxKey int

const userIDKey ctxKey = 0

// UserIDFrom extracts the authenticated user's ID from the request context.
func UserIDFrom(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(userIDKey).(int64)
	return id, ok
}

// Auth validates the JWT from the auth cookie and puts the user ID in context.
func Auth(jwtSecret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(authCookie)
			if err != nil {
				writeErr(w, http.StatusUnauthorized, "not authenticated")
				return
			}
			userID, err := security.ParseToken(jwtSecret, cookie.Value)
			if err != nil {
				writeErr(w, http.StatusUnauthorized, "invalid or expired session")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDKey, userID)))
		})
	}
}

// CSRF enforces the double-submit pattern on state-changing methods:
// the X-CSRF-Token header must match the csrf cookie set at login.
func CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		cookie, err := r.Cookie(csrfCookie)
		header := r.Header.Get("X-CSRF-Token")
		if err != nil || header == "" ||
			subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) != 1 {
			writeErr(w, http.StatusForbidden, "CSRF token missing or invalid")
			return
		}
		next.ServeHTTP(w, r)
	})
}
