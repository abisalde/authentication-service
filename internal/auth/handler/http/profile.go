package http

import (
	"context"

	"github.com/abisalde/authentication-service/internal/auth"
	"github.com/abisalde/authentication-service/internal/auth/service"
	"github.com/abisalde/authentication-service/internal/graph/converters"
	"github.com/abisalde/authentication-service/internal/graph/errors"
	"github.com/abisalde/authentication-service/internal/graph/model"
	"github.com/abisalde/authentication-service/pkg/password"
)

type ProfileHandler struct {
	authService service.AuthService
}

func NewProfileHandler(authService service.AuthService) *ProfileHandler {
	return &ProfileHandler{authService: authService}
}

func (h *ProfileHandler) GetUserProfile(ctx context.Context) (*model.User, error) {
	currentUser := auth.GetCurrentUser(ctx)

	if currentUser == nil {
		return nil, errors.AuthenticationRequired
	}

	return converters.UserToGraph(currentUser), nil
}

func (h *ProfileHandler) HandlePasswordChange(ctx context.Context, input model.ChangePasswordInput) (bool, error) {
	_, err := password.VerifyPasswords(&input)
	if err != nil {
		return false, err
	}

	currentUser := auth.GetCurrentUser(ctx)

	if currentUser == nil {
		return false, errors.AuthenticationRequired
	}

	if err := password.CheckPasswordHash(input.OldPassword, currentUser.PasswordHash); err != nil {
		return false, errors.InvalidCredentialsPassword
	}

	newPasswordHash, err := password.HashPassword(input.NewPassword)

	if err != nil {
		return false, errors.ErrSomethingWentWrong
	}

	if err := h.authService.UpdateUserPassword(ctx, currentUser.ID, newPasswordHash); err != nil {
		return false, errors.ErrSomethingWentWrong
	}

	return true, nil
}
