package jwt

import (
	"errors"
	"fmt"
	"os"
	"time"

	customErrors "github.com/abisalde/authentication-service/internal/graph/errors"
	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID    int64  `json:"userId"`
	UserEmail string `json:"email"`
	Type      string `json:"type"` //access or refresh
	jwt.RegisteredClaims
}

const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

func GetJWTSecret() (string, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return "", customErrors.JWTSecretNotConfigured
	}
	return secret, nil
}

func GenerateToken(userID int64, tokenType, email string, expiration time.Duration) (string, error) {
	if tokenType != TokenTypeAccess && tokenType != TokenTypeRefresh {
		return "", customErrors.InvalidTokenType
	}
	secret, err := GetJWTSecret()
	if err != nil {
		return "", err
	}

	claims := &Claims{
		UserID:    userID,
		Type:      tokenType,
		UserEmail: email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "authentication-service",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}
	return tokenString, nil
}

func ValidateToken(tokenString string) (*Claims, error) {

	secret, err := GetJWTSecret()
	if err != nil {
		return nil, err
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {

		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, customErrors.ExpiredToken
		}
		return nil, customErrors.InvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, customErrors.InvalidToken
	}

	if claims.Type != TokenTypeAccess && claims.Type != TokenTypeRefresh {
		return nil, customErrors.InvalidTokenType
	}

	return claims, nil
}

func (c *Claims) IsAccessToken() bool {
	return c.Type == TokenTypeAccess
}

func (c *Claims) IsRefreshToken() bool {
	return c.Type == TokenTypeRefresh
}

func GetTokenRemainingTTL(tokenString string) time.Duration {
	claims := &Claims{}
	_, _, err := new(jwt.Parser).ParseUnverified(tokenString, claims)
	if err != nil || claims.ExpiresAt == nil {
		return 0
	}
	return time.Until(claims.ExpiresAt.Time)
}
