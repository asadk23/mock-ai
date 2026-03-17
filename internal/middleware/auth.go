// Package middleware provides HTTP middleware for the mock-ai server.
// All middleware follows the func(next http.Handler) http.Handler pattern.
package middleware

import (
	"net/http"
	"strings"

	"github.com/asadk23/mock-ai/internal/api"
	"github.com/asadk23/mock-ai/internal/model"
)

// Auth returns middleware that validates Bearer token authentication.
// If requireToken is non-empty, the token must match exactly.
// If requireToken is empty, any non-empty Bearer token is accepted.
func Auth(enabled bool, requireToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !enabled {
				next.ServeHTTP(w, r)
				return
			}

			header := r.Header.Get("Authorization")
			if header == "" {
				api.WriteError(w, http.StatusUnauthorized,
					"You didn't provide an API key. You need to provide your API key in an Authorization header using Bearer auth (i.e. Authorization: Bearer YOUR_KEY).",
					model.ErrTypeAuth, model.ErrCodeInvalidAPIKey)
				return
			}

			token, found := strings.CutPrefix(header, "Bearer ")
			if !found || token == "" {
				api.WriteError(w, http.StatusUnauthorized,
					"Invalid Authorization header format. Expected 'Bearer <token>'.",
					model.ErrTypeAuth, model.ErrCodeInvalidAPIKey)
				return
			}

			if requireToken != "" && token != requireToken {
				maskedToken := token
				if len(maskedToken) > 3 {
					maskedToken = maskedToken[:3] + "***"
				} else {
					maskedToken = "***"
				}
				api.WriteError(w, http.StatusUnauthorized,
					"Incorrect API key provided: "+maskedToken+". You can find your API key at https://platform.openai.com/account/api-keys.",
					model.ErrTypeAuth, model.ErrCodeInvalidAPIKey)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
