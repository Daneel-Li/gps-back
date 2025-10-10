package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/Daneel-Li/gps-back/internal/config"
	"github.com/Daneel-Li/gps-back/internal/services"
)

type Middleware func(http.HandlerFunc) http.HandlerFunc

func WithMidWare(finalHandler http.HandlerFunc, middlwares ...Middleware) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f := finalHandler
		for _, m := range middlwares {
			f = m(f)
		}
		f(w, r)
	}
}

// Middleware is a middleware function for request validation
func ApiAuthCheck(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get API key from request
		providedKey := r.Header.Get("appKey")

		// Validate API key
		if providedKey != config.GetConfig().MxmAPIKey {
			// If key is incorrect, return error response
			http.Error(w, "Invalid appKey", http.StatusUnauthorized)
			return
		}

		// If key is correct, call next handler
		h.ServeHTTP(w, r)
	}
}

func JWTMiddleware(h http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Get Authorization token from request Header
		tokenString := r.Header.Get("Authorization")
		if len(tokenString) < 1 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		userid, err := services.NewJWTService().ValidateToken(tokenString)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		slog.Debug(fmt.Sprintf("[%s] %s userid:[%v]", r.Method, r.URL.Path, userid))
		ctx := context.WithValue(r.Context(), "userid", userid)
		r = r.WithContext(ctx)

		// Call next handler
		h.ServeHTTP(w, r)
	})
}
