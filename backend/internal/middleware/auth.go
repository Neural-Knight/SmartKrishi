package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/smartkrishi/backend/internal/api"
	"github.com/smartkrishi/backend/internal/domain"
	authservice "github.com/smartkrishi/backend/internal/service/auth"
)

type contextKeyAuth struct{}
type contextKeyUser struct{}

func bearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	if header == "" || !strings.HasPrefix(header, "Bearer ") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")), true
}

// Auth validates Bearer JWT and stores claims in context.
func Auth(authService *authservice.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r)
			if !ok {
				api.WriteError(w, http.StatusUnauthorized, "Could not validate credentials")
				return
			}

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

// AuthWithUser validates the Bearer JWT, resolves the owning user once, and
// stores both the claims and the *domain.User in context. Handlers that need
// the user's ID (e.g. chat routes) use UserFromContext to avoid a second DB
// lookup per request.
func AuthWithUser(authService *authservice.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r)
			if !ok {
				api.WriteError(w, http.StatusUnauthorized, "Could not validate credentials")
				return
			}

			claims, err := authService.VerifyToken(token)
			if err != nil {
				w.Header().Set("WWW-Authenticate", "Bearer")
				api.WriteError(w, http.StatusUnauthorized, "Could not validate credentials")
				return
			}

			user, err := authService.GetCurrentUser(r.Context(), claims)
			if err != nil {
				api.WriteError(w, http.StatusUnauthorized, "Could not validate credentials")
				return
			}
			if !user.IsActive {
				api.WriteError(w, http.StatusBadRequest, "Inactive user")
				return
			}

			ctx := context.WithValue(r.Context(), contextKeyAuth{}, claims)
			ctx = context.WithValue(ctx, contextKeyUser{}, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ClaimsFromContext returns JWT claims from request context.
func ClaimsFromContext(ctx context.Context) (*authservice.TokenClaims, bool) {
	claims, ok := ctx.Value(contextKeyAuth{}).(*authservice.TokenClaims)
	return claims, ok
}

// UserFromContext returns the resolved user stored by AuthWithUser.
func UserFromContext(ctx context.Context) (*domain.User, bool) {
	user, ok := ctx.Value(contextKeyUser{}).(*domain.User)
	return user, ok
}
