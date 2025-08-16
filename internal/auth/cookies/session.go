package cookies

import (
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
)

var (
	BrowserSessionTokenName = "authentication_service_session_token"
	BrowserAccessTokenName  = "authentication_service_access_token"
)

func CreateBrowserSession(generatedTokens TokenPair, ctx *fiber.Ctx) error {

	isProd := os.Getenv("APP_ENV") == "production"

	site := fiber.CookieSameSiteLaxMode

	if !isProd {
		site = fiber.CookieSameSiteStrictMode
	}

	refreshTokenExpiration := time.Now().Add(RefreshTokenExpiry)
	accessTokenExpiration := time.Now().Add(LoginAccessTokenExpiry)

	ctx.Cookie(&fiber.Cookie{
		Secure:   isProd,
		Expires:  refreshTokenExpiration,
		Name:     BrowserSessionTokenName,
		Value:    generatedTokens.RefreshToken,
		SameSite: site,
		Path:     "/",
		MaxAge:   int(time.Until(refreshTokenExpiration).Seconds()),
	})

	ctx.Cookie(&fiber.Cookie{
		Secure:   isProd,
		Expires:  accessTokenExpiration,
		Name:     BrowserAccessTokenName,
		Value:    generatedTokens.AccessToken,
		SameSite: site,
		Path:     "/",
		MaxAge:   int(time.Until(accessTokenExpiration).Seconds()),
	})

	return nil
}
