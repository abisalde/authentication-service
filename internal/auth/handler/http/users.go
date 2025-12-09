package http

import (
	"context"

	"github.com/abisalde/authentication-service/internal/auth/service"
	"github.com/abisalde/authentication-service/internal/graph/model"
)

type UsersHandler struct {
	authService *service.AuthService
}

func NewUsersHandler(authService *service.AuthService) *UsersHandler {
	return &UsersHandler{authService: authService}
}

func (h *UsersHandler) GetAllUsers(ctx context.Context, role *model.UserRole, first *int, after *string) (*model.UserConnection, error) {

	pagination := &model.PaginationInput{
		Limit: first,
		After: after,
	}

	return h.authService.FindUsers(ctx, role, pagination)
}
