package middleware

import (
	"github.com/abisalde/authentication-service/internal/auth"
	"github.com/gofiber/fiber/v2"
)

func OAuthStateMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		state := c.Query("state")
		if state != "" {
			c.Locals(auth.OAuthStateKey, state)
		}
		return c.Next()
	}
}
