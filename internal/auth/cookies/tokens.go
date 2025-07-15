package cookies

import (
	"time"

	"github.com/abisalde/authentication-service/pkg/jwt"
)

const (
	AccessTokenExpiry  = 24 * time.Hour
	RefreshTokenExpiry = 7 * 24 * time.Hour
)

type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

func GenerateTokenPair(userID int64, email string) (TokenPair, error) {
	accessToken, err := jwt.GenerateToken(userID, jwt.TokenTypeAccess, email, AccessTokenExpiry)
	if err != nil {
		return TokenPair{}, err
	}

	refreshToken, err := jwt.GenerateToken(userID, jwt.TokenTypeRefresh, email, RefreshTokenExpiry)

	if err != nil {
		return TokenPair{}, err
	}
	return TokenPair{AccessToken: accessToken, RefreshToken: refreshToken}, nil
}
