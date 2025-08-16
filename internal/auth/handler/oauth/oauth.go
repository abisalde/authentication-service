package oauth

import (
	"context"
	"errors"
	"log"
	"os"
	"strings"
	"time"

	"github.com/abisalde/authentication-service/internal/auth"
	"github.com/abisalde/authentication-service/internal/auth/cookies"
	"github.com/abisalde/authentication-service/internal/auth/service"
	"github.com/abisalde/authentication-service/internal/graph/model"
	oauthPKCE "github.com/abisalde/authentication-service/pkg/oauth"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type OAuthHandler struct {
	oauthService *service.OAuthService
}

func NewOAuthHandler(oauthService *service.OAuthService) *OAuthHandler {
	return &OAuthHandler{oauthService: oauthService}
}

func (h *OAuthHandler) InitOAuth(ctx context.Context, input model.OAuthLoginInput) (*model.PasswordLessResponse, error) {
	isProd := os.Getenv("APP_ENV") == "production"
	stateUUID := uuid.NewString()

	ctx = context.WithValue(ctx, auth.OAuthModeKey, input.Mode)
	ctx = context.WithValue(ctx, auth.OAuthPlatformKey, input.Platform)

	platform, ok := ctx.Value(auth.OAuthPlatformKey).(model.OAuthPlatform)

	if !ok {
		return nil, errors.New("can't start the oauth flow")
	}

	mode, _ := ctx.Value(auth.OAuthModeKey).(model.PasswordLessMode)

	authURL, state, err := h.oauthService.GetAuthPKCEURL(ctx, string(input.Provider), platform, stateUUID, mode)
	if err != nil {
		return nil, err
	}

	ctx = context.WithValue(ctx, auth.OAuthStateKey, state)
	ctx = context.WithValue(ctx, auth.OAuthUUIDKey, stateUUID)
	if fiberCtx, ok := ctx.Value(auth.FiberContextWeb).(*fiber.Ctx); ok {
		fiberCtx.Locals(auth.OAuthStateKey, state)
		fiberCtx.Locals(ctx, auth.OAuthUUIDKey, stateUUID)
	}

	response := &model.PasswordLessResponse{
		AuthURL: authURL,
	}

	if platform == model.OAuthPlatformMobile {
		response.StateKey = stateUUID
	}

	if platform == model.OAuthPlatformWeb {
		if fiberCtx, ok := ctx.Value(auth.FiberContextWeb).(*fiber.Ctx); ok {
			fiberCtx.Set("Cross-Origin-Opener-Policy", "same-origin-allow-popups")
			fiberCtx.Cookie(&fiber.Cookie{
				Secure:   isProd,
				Name:     string(auth.OAuthUUIDKey),
				Value:    stateUUID,
				HTTPOnly: true,
				SameSite: "Lax",
				Path:     "/",
				MaxAge:   int((11 * time.Minute).Seconds()),
			})
		} else {
			log.Printf("⚠️ Fiber context not available, can't set cookie for state")
		}
	}

	return response, nil
}

func (h *OAuthHandler) UnifiedOauthCallBack(c *fiber.Ctx) error {
	var (
		expectedState    string
		platform         model.OAuthPlatform
		cookiesStateUUID string
	)

	provider := strings.ToLower(c.Params("provider"))
	state := c.Query("state")
	code := c.Query("code")

	if val := c.Locals(auth.OAuthStateKey); val != nil {
		log.Printf("Expected state from Locals: %s", expectedState)
		expectedState = val.(string)
	}

	cookiesStateUUID = c.Cookies(string(auth.OAuthUUIDKey))
	if val := c.Locals(auth.OAuthUUIDKey); val != nil {
		cookiesStateUUID = val.(string)
	}

	platform = model.OAuthPlatformWeb
	if val := c.Locals(auth.OAuthPlatformKey); val != nil {
		log.Printf("Expected auth.OAuthPlatformKey from Locals: %s", val)
		if platformStr, ok := val.(string); ok {
			switch platformStr {
			case string(model.OAuthPlatformMobile):
				platform = model.OAuthPlatformMobile
			case string(model.OAuthPlatformWeb):
				platform = model.OAuthPlatformWeb
			default:
				log.Printf("Unknown platform override: %s. Keeping default: %s", platformStr, platform)
			}
		}
	}

	if state != expectedState || state == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid Authentication",
			"message": "Please try again with your request",
		})
	}

	log.Printf("This is the state UUID %s", cookiesStateUUID)

	stateUUID, platformKey, mode, err := oauthPKCE.DecodeState(state)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid state format",
			"message": "Invalid authentication request, try again",
		})
	}

	if stateUUID == "" || platformKey == "" || mode == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid CSRF state",
			"message": "Authentication failed. Please try again.",
		})
	}

	platform = model.OAuthPlatform(platformKey)
	passwordLessMode := model.PasswordLessMode(mode)

	tokens, user, platform, err := h.oauthService.HandleCallBack(c, provider, string(platform), string(passwordLessMode), code, stateUUID)

	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	ctx := context.Background()
	if platform == model.OAuthPlatformWeb {
		if fiberCtx, ok := ctx.Value(auth.FiberContextWeb).(*fiber.Ctx); ok {
			err = cookies.CreateBrowserSession(cookies.TokenPair{
				AccessToken:  tokens.AccessToken,
				RefreshToken: tokens.RefreshToken,
			}, fiberCtx)
			if err != nil {
				return errors.New("something went wrong try again")
			}
		}
		c.Cookies(string(auth.OAuthUUIDKey), "")
		redirectURL := h.oauthService.GetFrontEndRedirectURL(platform, tokens.AccessToken, user.Email)
		c.Set("Cross-Origin-Opener-Policy", "same-origin-allow-popups")
		return c.Redirect(redirectURL, fiber.StatusTemporaryRedirect)
	}

	if platform == model.OAuthPlatformMobile {
		redirectURL := h.oauthService.GetFrontEndRedirectURL(platform, tokens.AccessToken, user.Email)
		return c.Redirect(redirectURL, fiber.StatusTemporaryRedirect)
	}

	return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
		"error":   "Service Unavailable",
		"message": "Unable to process the request at this time.",
	})
}
