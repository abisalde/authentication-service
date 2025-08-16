package http

import (
	"context"
	"log"

	"github.com/abisalde/authentication-service/internal/auth/cookies"
	"github.com/abisalde/authentication-service/internal/auth/service"
	"github.com/abisalde/authentication-service/internal/graph/errors"
	"github.com/abisalde/authentication-service/internal/graph/model"
)

type TokenHandler struct {
	authService service.AuthService
}

func NewTokenHandler(authService service.AuthService) *TokenHandler {
	return &TokenHandler{authService: authService}
}

func (h *TokenHandler) HandleRefreshToken(
	ctx context.Context, token string, uid int32,
) (*model.RefreshTokenResponse, error) {

	userID := int64(uid)

	ok, err := h.authService.ValidateRefreshToken(ctx, userID, token)
	if !ok {
		log.Printf("Error from validating refresh token: %v", err)
		return nil, errors.InvalidRefreshTokenValidation
	}

	err = h.authService.CheckIfRefreshTokenMatchClaims(ctx, userID)
	log.Printf("h.authService.CheckIfRefreshTokenMatchClaims %v", err)
	if err != nil {
		return nil, err
	}

	accessToken, err := cookies.GenerateAccessToken(userID)
	if err != nil {
		log.Printf("Error from generating access token: %v", err)
		return nil, errors.AccessTokenGeneration
	}

	return &model.RefreshTokenResponse{
		Token: accessToken,
	}, nil
}
