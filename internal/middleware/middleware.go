package middleware

import (
	"context"
	"github.com/fortuna91/ya_praktikum_final/internal/auth"
	"github.com/fortuna91/ya_praktikum_final/internal/handlers"
	"net/http"
)

func Authorization(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/user/register" || r.URL.Path == "/api/user/login" {
			next.ServeHTTP(w, r)
			return
		}

		token, err := auth.GetTokenFromHeader(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		login, errParse := auth.ParseToken(token)
		if errParse != nil {
			http.Error(w, errParse.Error(), http.StatusUnauthorized)
			return
		}
		if user := handlers.DB.GetUser(context.Background(), login); user == nil {
			http.Error(w, "unknown user", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
