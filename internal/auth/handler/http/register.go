package http

import (
	"context"
	"time"

	"github.com/abisalde/authentication-service/internal/auth/service"
	"github.com/abisalde/authentication-service/internal/graph/errors"
	"github.com/abisalde/authentication-service/internal/graph/model"
	"github.com/abisalde/authentication-service/pkg/password"
	"github.com/abisalde/authentication-service/pkg/verification"
)

type RegisterHandler struct {
	authService service.AuthService
}

func NewRegisterHandler(authService service.AuthService) *RegisterHandler {
	return &RegisterHandler{authService: authService}
}

func (h *RegisterHandler) Register(ctx context.Context, input model.RegisterInput) (*model.RegisterResponse, error) {

	emailExist, err := h.authService.InitiateRegistration(ctx, input)
	if err != nil {
		return nil, errors.ErrSomethingWentWrong
	}

	if emailExist {
		return nil, errors.EmailExists
	}

	hashedPassword, err := password.HashPassword(input.Password)
	if err != nil {
		return nil, errors.ErrSomethingWentWrong
	}

	code := verification.GenerateVerificationCode()
	expiresAt := time.Now().Add(5 * time.Minute)

	pendingUser := model.PendingUser{
		Email:            input.Email,
		HashPassword:     hashedPassword,
		VerificationCode: code,
		CreatedAt:        time.Now(),
		ExpiresAt:        expiresAt,
	}

	err = h.authService.CreatePendingUser(ctx, pendingUser)

	if err != nil {
		return nil, errors.ErrSomethingWentWrong
	}

	if err := h.authService.SendVerificationCodeEmail(ctx, pendingUser.Email, code); err != nil {
		_ = h.authService.DeletePendingUser(ctx, pendingUser.Email)
		_ = h.authService.CleanupTemporaryData(ctx, pendingUser.Email)
		return nil, errors.ErrSomethingWentWrong
	}

	return &model.RegisterResponse{
		User: model.PublicUser{
			Email: input.Email,
		},
		Message: "Verification code sent to your email",
	}, nil
}

func (h *RegisterHandler) VerifyUserEmail(ctx context.Context, input model.AccountVerification) (bool, error) {
	user, err := h.authService.VerifyCodeAndCreateUser(ctx, input.Email, input.Code)
	if err != nil {
		return false, err
	}

	if user == nil {
		return false, errors.EmailVerificationFailed
	}

	return true, nil
}

func (h *RegisterHandler) ResendVerificationCodeEmail(ctx context.Context, input model.ResendVerificationCode) (bool, error) {

	pendingUser, err := h.authService.GetPendingUser(ctx, input.Email)
	if err != nil {
		return false, errors.UserNotFound
	}

	newCode := verification.GenerateVerificationCode()
	newExpiration := time.Now().Add(5 * time.Minute)

	pendingUser.VerificationCode = newCode
	pendingUser.ExpiresAt = newExpiration

	if err := h.authService.UpdatePendingUser(ctx, *pendingUser); err != nil {
		return false, errors.ErrSomethingWentWrong
	}

	if err := h.authService.SendVerificationCodeEmail(ctx, pendingUser.Email, newCode); err != nil {
		return false, errors.ErrSomethingWentWrong
	}

	return true, nil
}
