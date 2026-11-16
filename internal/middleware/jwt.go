package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTManager struct {
	secret []byte
}

type userIDContextKey struct{}

func NewJWTManager(secret string) (*JWTManager, error) {
	if len(secret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 bytes")
	}

	return &JWTManager{secret: []byte(secret)}, nil
}

func (m *JWTManager) Generate(userID int64) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   fmt.Sprint(userID),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func (m *JWTManager) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "missing or invalid bearer token", http.StatusUnauthorized)
			return
		}

		claims := &jwt.RegisteredClaims{}
		token, err := jwt.ParseWithClaims(parts[1], claims, func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, errors.New("unexpected signing method")
			}
			return m.secret, nil
		}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
		if err != nil || !token.Valid {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "invalid or expired bearer token", http.StatusUnauthorized)
			return
		}

		userID, err := strconv.ParseInt(claims.Subject, 10, 64)
		if err != nil || userID <= 0 || claims.ExpiresAt == nil {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "invalid bearer token", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userIDContextKey{}, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *JWTManager) AuthenticateOptional(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimSpace(r.Header.Get("Authorization")) == "" {
			next.ServeHTTP(w, r)
			return
		}
		m.Authenticate(next).ServeHTTP(w, r)
	})
}

func UserID(ctx context.Context) (int64, bool) {
	userID, ok := ctx.Value(userIDContextKey{}).(int64)
	return userID, ok
}
