package oauthPKCE

import (
	"fmt"
	"strings"

	"github.com/abisalde/authentication-service/internal/graph/model"
)

func EncodeState(uuid string, platform model.OAuthPlatform, mode model.PasswordLessMode) string {
	return fmt.Sprintf("%s|%s|%s", uuid, platform, mode)
}

func DecodeState(state string) (uuid, platform, mode string, err error) {
	parts := strings.Split(state, "|")
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid state format")
	}
	return parts[0], parts[1], parts[2], nil
}
