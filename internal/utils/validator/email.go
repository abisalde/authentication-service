package validator

import (
	"errors"
	"regexp"

	"github.com/abisalde/authentication-service/internal/graph/model"
	"github.com/vektah/gqlparser/v2/gqlerror"

	customErrors "github.com/abisalde/authentication-service/internal/graph/errors"
)

var (
	emailRegex      = regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,4}$`)
	ErrInvalidEmail = errors.New("invalid email format")
)

func ValidateEmail(email string) *gqlerror.Error {
	if !emailRegex.MatchString(email) {
		return customErrors.NewTypedError(ErrInvalidEmail.Error(), model.ErrorTypeEmail, nil)
	}
	return nil
}
