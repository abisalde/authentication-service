package http

import (
	"context"
	"log"
	"net/http"

	"github.com/abisalde/authentication-service/internal/auth"
	"github.com/abisalde/authentication-service/internal/auth/cookies"
	"github.com/abisalde/authentication-service/internal/auth/service"
	"github.com/abisalde/authentication-service/internal/graph/errors"
	"github.com/abisalde/authentication-service/internal/graph/model"
	"github.com/abisalde/authentication-service/pkg/jwt"
	"github.com/abisalde/authentication-service/pkg/password"
	"github.com/gofiber/fiber/v2"
)

type LoginHandler struct {
	authService *service.AuthService
}

func NewLoginHandler(authService *service.AuthService) *LoginHandler {
	return &LoginHandler{authService: authService}
}

func (h *LoginHandler) EmailLogin(ctx context.Context, input model.LoginInput) (*model.LoginResponse, error) {

	user, err := h.authService.InitiateLogin(ctx, input.Email)
	if err != nil {
		return nil, errors.InvalidCredentialsEmail
	}

	err = password.CheckPasswordHash(input.Password, user.PasswordHash)
	if err != nil {
		return nil, errors.InvalidCredentialsPassword
	}

	tokens, err := cookies.GenerateLoginTokenPair(user.ID)

	if err != nil {
		log.Printf("This is error from cookies.GenerateLoginTokenPair: %v", err)
		return nil, errors.ErrSomethingWentWrong
	}

	// Store and Hash the RefreshToken
	hashedToken, refreshErr := h.authService.StoreRefreshToken(ctx, user.ID, tokens.RefreshToken)

	if refreshErr != nil {
		return nil, errors.ErrSomethingWentWrong
	}

	if fiberCtx, ok := ctx.Value(auth.FiberContextWeb).(*fiber.Ctx); ok {
		err = cookies.CreateBrowserSession(cookies.TokenPair{
			AccessToken:  tokens.AccessToken,
			RefreshToken: hashedToken,
		}, fiberCtx)
		if err != nil {
			return nil, errors.ErrSomethingWentWrong
		}
	}

	err = h.authService.UpdateLastLogin(ctx, user.ID)
	if err != nil {
		return nil, errors.ErrSomethingWentWrong
	}

	return &model.LoginResponse{
		UserId:       user.ID,
		Token:        tokens.AccessToken,
		RefreshToken: hashedToken,
		Email:        user.Email,
	}, nil
}

func (h *LoginHandler) ProcessLogout(ctx context.Context) (bool, error) {
	currentUser := auth.GetCurrentUser(ctx)
	if currentUser == nil {
		return false, errors.AuthenticationRequired
	}

	_ = h.authService.InvalidateRefreshToken(ctx, currentUser.ID)

	token, ok := ctx.Value(auth.JWTTokenKey).(string)
	if ok && token != "" {
		remainingTTL := jwt.GetTokenRemainingTTL(token)
		if remainingTTL > 0 {
			h.authService.BlacklistToken(ctx, token, remainingTTL)
		}
	}

	if fiberCtx, ok := ctx.Value(auth.FiberContextWeb).(*fiber.Ctx); ok {
		fiberCtx.ClearCookie(cookies.BrowserAccessTokenName)
		fiberCtx.ClearCookie(cookies.BrowserSessionTokenName)
	}
	if w, ok := ctx.Value(auth.HTTPResponseWriterKey).(http.ResponseWriter); ok {
		http.SetCookie(w, &http.Cookie{Name: cookies.BrowserAccessTokenName, MaxAge: -1, Path: "/"})
		http.SetCookie(w, &http.Cookie{Name: cookies.BrowserSessionTokenName, MaxAge: -1, Path: "/"})
	}

	return true, nil
}
