package service

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const UserIDKey contextKey = "userID"
const AdminUsernameKey contextKey = "adminUsername"

func parseTokenClaims(auth string) (jwt.MapClaims, error) {
	if strings.TrimSpace(auth) == "" {
		return nil, errors.New("missing authorization header")
	}

	tokenStr := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	if tokenStr == "" || tokenStr == auth {
		return nil, errors.New("invalid authorization header")
	}

	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		return []byte(os.Getenv("JWT_SECRET")), nil
	})
	if err != nil || !token.Valid {
		return nil, errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid token claims")
	}

	return claims, nil
}

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := parseTokenClaims(r.Header.Get("Authorization"))
		if err != nil {
			Error(w, 9001, "unauthorized")
			return
		}

		openID, ok := claims["openid"].(string)
		if !ok || strings.TrimSpace(openID) == "" {
			Error(w, 9001, "unauthorized")
			return
		}

		ctx := context.WithValue(r.Context(), UserIDKey, openID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func AdminAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := parseTokenClaims(r.Header.Get("Authorization"))
		if err != nil {
			Error(w, 9101, "unauthorized")
			return
		}

		role, ok := claims["role"].(string)
		if !ok || role != "admin" {
			Error(w, 9102, "admin only")
			return
		}

		username, _ := claims["username"].(string)
		ctx := context.WithValue(r.Context(), AdminUsernameKey, username)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
