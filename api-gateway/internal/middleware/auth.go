package middleware

import (
	"net/http"
	"strings"

	"github.com/DoMinhHHung/beester/api-gateway/internal/auth"
	"github.com/DoMinhHHung/beester/api-gateway/internal/identity"
)

func JWTAuth(
	validator *auth.Validator,
	publicPathPrefixes []string,
	userIDHeader string,
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if userIDHeader != "" {
			r.Header.Del(userIDHeader)
		}

		if pathIsPublic(r.URL.Path, publicPathPrefixes) {
			next.ServeHTTP(w, r)
			return
		}

		tokenString, ok := bearerToken(r.Header.Get("Authorization"))
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		userID, err := validator.Validate(tokenString)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := identity.WithContext(r.Context(), identity.Identity{UserID: userID})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func bearerToken(value string) (string, bool) {
	parts := strings.Fields(value)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

func pathIsPublic(path string, prefixes []string) bool {
	for _, prefix := range prefixes {
		prefix = strings.TrimSpace(prefix)
		if prefix == "" {
			continue
		}
		if strings.HasSuffix(prefix, "/") {
			if strings.HasPrefix(path, prefix) {
				return true
			}
			continue
		}
		if path == prefix {
			return true
		}
	}
	return false
}
