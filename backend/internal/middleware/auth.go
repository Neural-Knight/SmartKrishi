package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/smartkrishi/backend/internal/api"
	authservice "github.com/smartkrishi/backend/internal/service/auth"
)

type contextKeyAuth struct{}

// Auth validates Bearer JWT and stores claims in context.
func Auth(authService *authservice.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" || !strings.HasPrefix(header, "Bearer ") {
				api.WriteError(w, http.StatusUnauthorized, "Could not validate credentials")
				return
			}

			token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
			claims, err := authService.VerifyToken(token)
			if err != nil {
				w.Header().Set("WWW-Authenticate", "Bearer")
				api.WriteError(w, http.StatusUnauthorized, "Could not validate credentials")
				return
			}

			ctx := context.WithValue(r.Context(), contextKeyAuth{}, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ClaimsFromContext returns JWT claims from request context.
func ClaimsFromContext(ctx context.Context) (*authservice.TokenClaims, bool) {
	claims, ok := ctx.Value(contextKeyAuth{}).(*authservice.TokenClaims)
	return claims, ok
}
