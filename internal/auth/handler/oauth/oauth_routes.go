package oauth

import (
	"github.com/abisalde/authentication-service/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

func (h *OAuthHandler) RegisterRoutes(appService *fiber.App) {
	oauthGroup := appService.Group("/service/oauth")
	oauthGroup.Get("/:provider/callback",
		middleware.OAuthStateMiddleware(),
		h.UnifiedOauthCallBack,
	)
}
