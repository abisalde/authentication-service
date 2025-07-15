package validator

import (
	"errors"
	"regexp"

	customErrors "github.com/abisalde/authentication-service/internal/graph/errors"
	"github.com/abisalde/authentication-service/internal/graph/model"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

var (
	ErrShortPassword         = errors.New("password must be at least 8 characters long")
	ErrorPasswordCombination = errors.New("password must contain one uppercase, one lowercase, one number, and one special character")
)

func ValidatePassword(password string) *gqlerror.Error {
	if len(password) < 8 {
		return customErrors.NewTypedError(ErrShortPassword.Error(), model.ErrorTypePassword, nil)
	}
	if !regexp.MustCompile(`[A-Z]`).MatchString(password) {
		return customErrors.NewTypedError(ErrorPasswordCombination.Error(), model.ErrorTypePassword, nil)
	}
	if !regexp.MustCompile(`[a-z]`).MatchString(password) {
		return customErrors.NewTypedError(ErrorPasswordCombination.Error(), model.ErrorTypePassword, nil)
	}
	if !regexp.MustCompile(`[0-9]`).MatchString(password) {
		return customErrors.NewTypedError(ErrorPasswordCombination.Error(), model.ErrorTypePassword, nil)
	}
	if !regexp.MustCompile(`[!@#~$%^&*()_+\-=\[\]{};':"\\|,.<>\/?]`).MatchString(password) {
		return customErrors.NewTypedError(ErrorPasswordCombination.Error(), model.ErrorTypePassword, nil)
	}
	return nil
}
