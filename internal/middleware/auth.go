package middleware

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/abisalde/authentication-service/internal/auth"
	"github.com/abisalde/authentication-service/internal/auth/cookies"
	"github.com/abisalde/authentication-service/internal/auth/service"
	"github.com/abisalde/authentication-service/internal/database/ent"
	customErrors "github.com/abisalde/authentication-service/internal/graph/errors"
	"github.com/abisalde/authentication-service/pkg/jwt"
	"github.com/gofiber/fiber/v2"
)

func AuthMiddleware(db *ent.Client, authService *service.AuthService) func(http.Handler) http.Handler {
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
				if err == nil && claims.IsAccessToken() {
					user, err := db.User.Get(ctx, claims.UserID)
					if err == nil {
						ctx = context.WithValue(ctx, auth.CurrentUserKey, user)
						ctx = context.WithValue(ctx, auth.ClientIPKey, r.RemoteAddr)
					}
				}
			}
			auth.DebugContext(ctx)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func Middleware(db *ent.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {

		if c.UserContext() == nil {
			c.SetUserContext(context.Background())
		}

		tokenString, err := stripToken(c)

		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		if tokenString == "" {
			if fiberCtx, ok := auth.GetFiberWebContext(c.Context()); ok {
				tokenString = fiberCtx.Cookies(cookies.BrowserAccessTokenName)
			}
		}

		claims, err := jwt.ValidateToken(tokenString)
		if err != nil {
			status := fiber.StatusUnauthorized
			if err == customErrors.ExpiredToken {
				status = fiber.StatusForbidden
			}
			return c.Status(status).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		if !claims.IsAccessToken() {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Refresh tokens cannot be used for authentication",
			})
		}

		user, err := db.User.Get(c.Context(), claims.UserID)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "User not found",
			})
		}

		ctx := c.UserContext()
		ctx = context.WithValue(ctx, auth.CurrentUserKey, user)
		ctx = context.WithValue(ctx, auth.ClientIPKey, c.IP())
		ctx = context.WithValue(ctx, auth.FiberContextWeb, c)

		auth.DebugContext(ctx)

		c.SetUserContext(ctx)
		return c.Next()
	}
}

func stripToken(c *fiber.Ctx) (string, error) {
	authHeader := c.Get("Authorization")
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

	return c.Cookies(cookies.BrowserAccessTokenName), nil
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
