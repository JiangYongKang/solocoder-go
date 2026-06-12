package gateway

import (
	"net/http"
	"strings"
)

type AuthMiddleware struct {
	store      TokenStore
	exemptPaths map[string]bool
}

func NewAuthMiddleware(store TokenStore) *AuthMiddleware {
	return &AuthMiddleware{
		store:      store,
		exemptPaths: make(map[string]bool),
	}
}

func (a *AuthMiddleware) ExemptPath(path string) {
	a.exemptPaths[path] = true
}

func (a *AuthMiddleware) Middleware() Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if a.exemptPaths[r.URL.Path] {
				next(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte("Unauthorized: Missing Authorization header"))
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte("Unauthorized: Invalid Authorization format"))
				return
			}

			token := parts[1]
			user, valid := a.store.Validate(token)
			if !valid {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte("Unauthorized: Invalid token"))
				return
			}

			ctx := WithUser(r.Context(), user)
			r = r.WithContext(ctx)

			next(w, r)
		}
	}
}
