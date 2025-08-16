package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/abisalde/authentication-service/internal/auth/cookies"
	"github.com/abisalde/authentication-service/internal/configs"
	"github.com/abisalde/authentication-service/internal/database/ent"
	"github.com/abisalde/authentication-service/internal/graph/errors"
	"github.com/abisalde/authentication-service/internal/graph/model"
	oauthPKCE "github.com/abisalde/authentication-service/pkg/oauth"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/facebook"
	"golang.org/x/oauth2/google"
)

type OAuthService struct {
	googleOAuthConfig   *oauth2.Config
	facebookOAuthConfig *oauth2.Config
	authService         *AuthService
}

func NewOAuthService(authService *AuthService) *OAuthService {
	return &OAuthService{
		googleOAuthConfig: &oauth2.Config{
			ClientID:     authService.cfg.Providers.GoogleClientID,
			ClientSecret: authService.cfg.Providers.GoogleClientSecret,
			RedirectURL:  GetRedirectUrl(authService.cfg, "google"),
			Scopes:       []string{"email", "profile"},
			Endpoint:     google.Endpoint,
		},

		facebookOAuthConfig: &oauth2.Config{
			ClientID:     authService.cfg.Providers.FBClientID,
			ClientSecret: authService.cfg.Providers.FBClientSecret,
			RedirectURL:  GetRedirectUrl(authService.cfg, "facebook"),
			Scopes:       []string{"email"},
			Endpoint:     facebook.Endpoint,
		},

		authService: authService,
	}
}

func GetRedirectUrl(cfg *configs.Config, provider string) string {
	provider = strings.ToLower(provider)
	var baseApiUrl string
	if cfg.Env.CurrentEnv == "production" {
		baseApiUrl = cfg.Env.BaseAPIUrl
	} else {
		baseApiUrl = "http://localhost:8080"
	}
	return fmt.Sprintf("%s/service/oauth/%s/callback", baseApiUrl, provider)
}

func (s *OAuthService) GetFrontEndRedirectURL(platform model.OAuthPlatform, token, email string) string {
	cfg := s.authService.cfg
	var redirectURL string
	frontendURL := "https://authentication-service.netlify.app"

	if platform == model.OAuthPlatformMobile {
		return fmt.Sprintf("nativeoauthgraphql://passwordless-authentication?token=%s&email=%s", token, email)
	}

	if platform == model.OAuthPlatformWeb {
		if cfg.Env.CurrentEnv == "production" {
			redirectURL = fmt.Sprintf("%s/saml/passwordless-authentication?token=%s&email=%s", frontendURL, token, email)
		} else {
			redirectURL = fmt.Sprintf("http://localhost:3000/saml/passwordless-authentication?token=%s&email=%s", token, email)
		}
		return redirectURL
	}

	if cfg.Env.CurrentEnv == "production" {
		redirectURL = frontendURL
	} else {
		redirectURL = "http://localhost:3000"
	}
	return redirectURL
}

func (s *OAuthService) GetAuthPKCEURL(ctx context.Context, provider string, platform model.OAuthPlatform, stateUUID string, mode model.PasswordLessMode) (string, string, error) {
	verifier := oauth2.GenerateVerifier()

	state := oauthPKCE.EncodeState(stateUUID, platform, mode)

	var authURL string

	cacheKey := fmt.Sprintf("oauth:%s:%s", platform, stateUUID)
	if err := s.authService.cache.Set(ctx, cacheKey, verifier, 10*time.Minute); err != nil {
		return "", "", errors.ErrSomethingWentWrong
	}

	switch model.OAuthProvider(provider) {
	case model.OAuthProviderGoogle:
		authURL = s.googleOAuthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.S256ChallengeOption(verifier))
	case model.OAuthProviderFacebook:
		authURL = s.facebookOAuthConfig.AuthCodeURL(state)
	default:
		return "", "", errors.ErrSomethingWentWrong
	}

	return authURL, state, nil
}

func (s *OAuthService) HandleCallBack(c *fiber.Ctx, provider, platform, mode, code, stateUUID string) (*cookies.TokenPair, *ent.User, model.OAuthPlatform, error) {
	cacheKey := fmt.Sprintf("oauth:%s:%s", platform, stateUUID)
	providerKey := strings.ToUpper(provider)

	ctx := c.Context()

	var codeVerifier string
	err := s.authService.cache.Get(ctx, cacheKey, &codeVerifier)

	if err != nil {
		return nil, nil, "", c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Time Elapsed",
			"message": "We couldn't complete your authentication at this time, please try again",
		})
	}

	var (
		config      *oauth2.Config
		userInfoURL string
		token       *oauth2.Token
		configErr   error
	)

	switch model.OAuthProvider(providerKey) {
	case model.OAuthProviderGoogle:
		config = s.googleOAuthConfig
		userInfoURL = "https://www.googleapis.com/oauth2/v2/userinfo"
		token, configErr = config.Exchange(ctx, code, oauth2.VerifierOption(codeVerifier))
	case model.OAuthProviderFacebook:
		config = s.facebookOAuthConfig
		userInfoURL = "https://graph.facebook.com/me?fields=id,name,email"
		token, configErr = config.Exchange(ctx, code)
	default:
		return nil, nil, "", c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid Provider",
			"message": "We couldn't find the provider at this time",
		})
	}

	if configErr != nil {
		return nil, nil, "", c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Authentication Exchange failed",
			"message": "We couldn't find the complete your authentication at this time",
		})
	}

	client := config.Client(ctx, token)
	response, err := client.Get(userInfoURL)

	if err != nil {
		return nil, nil, "", c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "User Authorization Failed",
			"message": "We could not find this user at this time, please try again with a different email",
		})
	}
	defer response.Body.Close()

	var userInfo *model.OAuthUserResponse
	if err := json.NewDecoder(response.Body).Decode(&userInfo); err != nil {
		return nil, nil, "", c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "User Profile fetching failed",
			"message": "We could not find this user at this time, please try again",
		})
	}

	var user *ent.User
	switch model.PasswordLessMode(mode) {

	case model.PasswordLessModeRegister:
		user, err = s.authService.userRepo.CreateUserFromOAuth(ctx, providerKey, userInfo)

	case model.PasswordLessModeLogin:
		userID := userInfo.ID
		user, err = s.authService.userRepo.FindByOAuthID(ctx, providerKey, userID)

	default:
		return nil, nil, "", c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid PasswordLess flow mode",
			"message": "Please try again with the right flow",
		})
	}

	if err != nil {
		return nil, nil, "", c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Something went wrong",
			"message": "Please try again",
		})
	}

	tokens, err := cookies.GenerateLoginTokenPair(int64(user.ID))
	if err != nil {
		return nil, nil, "", c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Something went wrong",
			"message": "Access token failure",
		})
	}

	hashedToken, refreshErr := s.authService.StoreRefreshToken(ctx, user.ID, tokens.RefreshToken)
	if refreshErr != nil {
		return nil, nil, "", c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Something went wrong",
			"message": "Failed to create a token",
		})
	}

	err = s.authService.UpdateLastLogin(ctx, user.ID)
	if err != nil {
		return nil, nil, "", errors.ErrSomethingWentWrong
	}

	tokePair := &cookies.TokenPair{
		AccessToken:  tokens.AccessToken,
		RefreshToken: hashedToken,
	}

	return tokePair, user, model.OAuthPlatform(platform), err
}
