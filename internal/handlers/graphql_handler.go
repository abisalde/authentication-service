package handlers

import (
	"context"
	"net/http"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/abisalde/authentication-service/internal/auth"
	app_logger "github.com/abisalde/authentication-service/pkg/logger"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
)

func GraphQLHandler(srv *handler.Server) fiber.Handler {
	return func(c *fiber.Ctx) error {
		clientIP := c.IP()
		remoteAddr := c.Context().RemoteAddr().String()
		app_logger.LogGraphQLRequest(clientIP, remoteAddr)

		ctx := context.WithValue(c.UserContext(), auth.ClientIPKey, remoteAddr)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(ctx)
			srv.ServeHTTP(w, r)
		})
		c.SetUserContext(ctx)
		return adaptor.HTTPHandler(handler)(c)
	}
}
