package middleware

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/abisalde/authentication-service/internal/auth"
	"github.com/abisalde/authentication-service/internal/auth/cookies"
	"github.com/abisalde/authentication-service/internal/auth/service"
	"github.com/abisalde/authentication-service/internal/database/ent"
	"github.com/abisalde/authentication-service/pkg/jwt"
	"github.com/abisalde/authentication-service/pkg/session"
)

func AuthMiddleware(db *ent.Client, authService *service.AuthService) func(http.Handler) http.Handler {
	// Initialize session manager for validation
	sessionManager := session.NewSessionManager(authService.GetCache().RawClient())
	
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			ctx = context.WithValue(ctx, auth.FiberContextWeb, r)
			ctx = context.WithValue(ctx, auth.HTTPResponseWriterKey, w)

			authHeader := r.Header.Get("Authorization")

			var tokenString string

			token, err := stripTokeContext(authHeader)

			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			tokenString = token

			if tokenString == "" {
				cookie, err := r.Cookie(cookies.BrowserAccessTokenName)
				if err == nil {
					tokenString = cookie.Value
				}
			}

			ctx = context.WithValue(ctx, auth.JWTTokenKey, tokenString)
			ctx = context.WithValue(ctx, auth.FiberContextWeb, r)

			if tokenString != "" {
				if authService.IsTokenBlacklisted(ctx, tokenString) {
					log.Println("Token is blacklisted")
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}

				claims, err := jwt.ValidateToken(tokenString)
				if err != nil {
					log.Printf("Token validation failed:  %v", err)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}

				if claims.IsAccessToken() {
					userID, parseErr := strconv.ParseInt(claims.Subject, 10, 64)

					if parseErr != nil {
						log.Printf("Invalid user ID in token claims: %v", parseErr)
						next.ServeHTTP(w, r.WithContext(ctx))
						return
					}

					// Validate session and update activity
					tokenHash := session.HashToken(tokenString)
					if sess, err := sessionManager.GetSessionByTokenHash(ctx, claims.Subject, tokenHash); err == nil {
						// Update session activity
						if err := sessionManager.UpdateSessionActivity(ctx, sess.SessionID); err != nil {
							log.Printf("Failed to update session activity: %v", err)
						}
						// Add session to context
						ctx = context.WithValue(ctx, auth.SessionInfoKey, sess)
					} else {
						log.Printf("Session not found for token, this might be an old token: %v", err)
					}

					user, err := db.User.Get(ctx, userID)
					if err == nil {
						ctx = context.WithValue(ctx, auth.CurrentUserKey, user)
						realClientIP := GetClientIP(r)
						ctx = context.WithValue(ctx, auth.ClientIPKey, realClientIP)
					}
				}
			}
			auth.DebugContext(ctx)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func stripTokeContext(authHeader string) (string, error) {
	authHeader = strings.TrimSpace(authHeader)
	if authHeader == "" {
		return "", errors.New("authorization header is empty")
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
		token := strings.TrimSpace(parts[1])
		if token == "" {
			return "", errors.New("token missing after Bearer")
		}
		return token, nil
	}

	return authHeader, nil
}

func GetClientIP(r *http.Request) string {

	log.Printf("This is the X-forwarded-Host I got here: %s", r.Header.Get("X-Forwarded-For"))

	if xForwardedFor := r.Header.Get("X-Forwarded-For"); xForwardedFor != "" {
		ips := strings.Split(xForwardedFor, ",")
		if len(ips) > 0 {

			return strings.TrimSpace(ips[0])
		}
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	log.Printf("This is the error from... SPLIT HOST..:%v", err)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
