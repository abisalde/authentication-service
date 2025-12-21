package session

import (
	"context"
	"net/http"
	"strings"
)

// ContextKey is the type used for context keys
type ContextKey string

const (
	// ContextKeyUserID is the context key for user ID
	ContextKeyUserID ContextKey = "userID"
	// ContextKeyTokenID is the context key for token ID
	ContextKeyTokenID ContextKey = "tokenID"
	// ContextKeyClaims is the context key for full claims
	ContextKeyClaims ContextKey = "claims"
)

// HTTPMiddleware creates an HTTP middleware that validates JWT tokens
// This middleware can be used by any microservice to protect endpoints
func HTTPMiddleware(validator *SessionValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Missing authorization header", http.StatusUnauthorized)
				return
			}

			// Remove "Bearer " prefix
			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			if tokenString == authHeader {
				// No "Bearer " prefix found
				http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
				return
			}

			// Validate token
			claims, err := validator.ValidateAccessToken(tokenString)
			if err != nil {
				http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
				return
			}

			// Add claims to context
			ctx := r.Context()
			ctx = context.WithValue(ctx, ContextKeyUserID, claims.GetUserID())
			ctx = context.WithValue(ctx, ContextKeyTokenID, claims.GetTokenID())
			ctx = context.WithValue(ctx, ContextKeyClaims, claims)

			// Call next handler with enriched context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalHTTPMiddleware creates an HTTP middleware that optionally validates JWT tokens
// If a valid token is present, user info is added to context
// If no token or invalid token, request continues without authentication
func OptionalHTTPMiddleware(validator *SessionValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			ctx := r.Context()

			if authHeader != "" {
				tokenString := strings.TrimPrefix(authHeader, "Bearer ")
				if tokenString != authHeader {
					claims, err := validator.ValidateAccessToken(tokenString)
					if err == nil {
						ctx = context.WithValue(ctx, ContextKeyUserID, claims.GetUserID())
						ctx = context.WithValue(ctx, ContextKeyTokenID, claims.GetTokenID())
						ctx = context.WithValue(ctx, ContextKeyClaims, claims)
					}
				}
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserID extracts user ID from request context
func GetUserID(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(ContextKeyUserID).(string)
	return userID, ok
}

// GetTokenID extracts token ID from request context
func GetTokenID(ctx context.Context) (string, bool) {
	tokenID, ok := ctx.Value(ContextKeyTokenID).(string)
	return tokenID, ok
}

// GetClaims extracts full claims from request context
func GetClaims(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(ContextKeyClaims).(*Claims)
	return claims, ok
}

// RequireAuth is a helper that can be used to wrap individual handlers
func RequireAuth(validator *SessionValidator, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Missing authorization header", http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
			return
		}

		claims, err := validator.ValidateAccessToken(tokenString)
		if err != nil {
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, ContextKeyUserID, claims.GetUserID())
		ctx = context.WithValue(ctx, ContextKeyTokenID, claims.GetTokenID())
		ctx = context.WithValue(ctx, ContextKeyClaims, claims)

		handler.ServeHTTP(w, r.WithContext(ctx))
	}
}
