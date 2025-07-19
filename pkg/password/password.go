package password

import (
	"strings"

	"github.com/abisalde/authentication-service/internal/graph/errors"
	"github.com/abisalde/authentication-service/internal/graph/model"
	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

func CheckPasswordHash(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func VerifyPasswords(input *model.ChangePasswordInput) (bool, error) {
	if input.NewPassword != input.ConfirmNewPassword {
		return false, errors.NewTypedError("New and Confirmation password do not match", model.ErrorTypePassword, map[string]interface{}{
			"NewPassword":     input.NewPassword,
			"ConfirmPassword": input.ConfirmNewPassword,
		})
	}
	if strings.EqualFold(input.OldPassword, input.NewPassword) {
		return false, errors.NewTypedError("New password must be different from old password", model.ErrorTypeWeakPassword, map[string]interface{}{
			"OldPassword": input.OldPassword,
			"NewPassword": input.NewPassword,
		})
	}

	return true, nil
}
