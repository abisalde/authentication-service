package http

import (
	"context"

	"github.com/abisalde/authentication-service/internal/auth"
	"github.com/abisalde/authentication-service/internal/auth/service"
	"github.com/abisalde/authentication-service/internal/graph/converters"
	"github.com/abisalde/authentication-service/internal/graph/errors"
	"github.com/abisalde/authentication-service/internal/graph/model"
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

	user, err := h.authService.FindUserProfileById(ctx, currentUser.ID)

	if err != nil {
		return nil, errors.ErrSomethingWentWrong
	}

	return converters.UserToGraph(user), nil
}
