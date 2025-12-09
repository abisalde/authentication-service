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
		return &model.User{}, errors.AuthenticationRequired
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

func (h *ProfileHandler) UpdateUserProfile(ctx context.Context, input model.UpdateProfileInput) (*model.User, error) {
	currentUser := auth.GetCurrentUser(ctx)
	if currentUser == nil {
		return nil, errors.AuthenticationRequired
	}

	// If username is being updated, check availability and update
	if input.Username != nil && *input.Username != "" {
		// Check if username already belongs to current user
		if currentUser.Username != *input.Username {
			// Check availability
			available, err := h.authService.CheckUsernameAvailability(ctx, *input.Username)
			if err != nil {
				return nil, err
			}
			if !available {
				return nil, errors.NewTypedError("Username is already taken", model.ErrorTypeBadRequest, map[string]interface{}{
					"field": "username",
				})
			}

			// Update username
			if err := h.authService.UpdateUsername(ctx, currentUser.ID, *input.Username); err != nil {
				return nil, err
			}
		}
	}

	// Get updated user and return
	updatedUser, err := h.authService.FindUserProfileById(ctx, currentUser.ID)
	if err != nil {
		return nil, err
	}

	return converters.UserToGraph(updatedUser), nil
}
