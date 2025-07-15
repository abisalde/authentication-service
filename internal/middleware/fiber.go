package middleware

import (
	"context"

	"github.com/abisalde/authentication-service/internal/auth"
	"github.com/gofiber/fiber/v2"
)

func FiberWebMiddleware(c *fiber.Ctx) error {
	ctx := context.WithValue(c.Context(), auth.FiberContextWeb, c)
	c.SetUserContext(ctx)
	return c.Next()
}
