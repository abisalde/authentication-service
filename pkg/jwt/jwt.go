package jwt

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	customErrors "github.com/abisalde/authentication-service/internal/graph/errors"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type TokenType string
type Claims struct {
	Type TokenType `json:"type"` //access or refresh
	jwt.RegisteredClaims
}

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

var (
	secretOnce sync.Once
	secretKey  []byte
	loadError  error

	issuer        = "authentication-service"
	clockSkew     = 30 * time.Second
	signingMethod = jwt.SigningMethodHS256
)

func loadSecret() error {
	secretOnce.Do(func() {
		val := os.Getenv("JWT_SECRET")
		if val == "" {
			loadError = errors.New("JWT secret not configured")
			return
		}
		secretKey = []byte(val)
	})
	return loadError
}

func GenerateToken(userID int64, tokenType TokenType, expiration time.Duration) (string, error) {
	if tokenType != TokenTypeAccess && tokenType != TokenTypeRefresh {
		return "", customErrors.InvalidTokenType
	}

	if err := loadSecret(); err != nil {
		return "", err
	}

	now := time.Now()
	sub := strconv.FormatInt(userID, 10)
	jti := uuid.NewString()

	claims := &Claims{
		Type: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			Subject:   sub,
			ExpiresAt: jwt.NewNumericDate(now.Add(expiration)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now.Add(-clockSkew)),
			Issuer:    issuer,
		},
	}

	token := jwt.NewWithClaims(signingMethod, claims)
	tokenString, err := token.SignedString(secretKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}
	return tokenString, nil
}

func ValidateToken(tokenString string) (*Claims, error) {

	if err := loadSecret(); err != nil {
		return nil, err
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {

		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secretKey, nil
	}, jwt.WithLeeway(clockSkew))

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
