package cookies

import (
	"time"

	"github.com/abisalde/authentication-service/pkg/jwt"
)

const (
	AccessTokenExpiry      = 12 * time.Hour
	RefreshTokenExpiry     = 15 * 24 * time.Hour
	LoginAccessTokenExpiry = 10 * time.Minute
)

type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

func GenerateAccessToken(userID int64) (string, error) {
	accessToken, err := jwt.GenerateToken(userID, jwt.TokenTypeAccess, AccessTokenExpiry)
	if err != nil {
		return "", err
	}

	return accessToken, nil
}

func GenerateLoginTokenPair(userID int64) (*TokenPair, error) {
	accessToken, err := jwt.GenerateToken(userID, jwt.TokenTypeAccess, LoginAccessTokenExpiry)

	if err != nil {
		return nil, err
	}

	refreshToken, err := jwt.GenerateToken(userID, jwt.TokenTypeRefresh, RefreshTokenExpiry)

	if err != nil {
		return nil, err
	}

	return &TokenPair{AccessToken: accessToken, RefreshToken: refreshToken}, nil
}
