package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/abisalde/authentication-service/internal/auth/cookies"
	"github.com/abisalde/authentication-service/internal/auth/repository"
	"github.com/abisalde/authentication-service/internal/configs"
	"github.com/abisalde/authentication-service/internal/database/ent"
	"github.com/abisalde/authentication-service/internal/graph/errors"
	"github.com/abisalde/authentication-service/internal/graph/model"
	"github.com/abisalde/authentication-service/pkg/jwt"
	"github.com/abisalde/authentication-service/pkg/mail"
	"github.com/abisalde/authentication-service/pkg/verification"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

const (
	LoginStreamKey     = "login_events"
	LoginGroup         = "login_event_group"
	RefreshCachePrefix = "refresh_token:"
)

type LoginEvent struct {
	UserID    int64     `json:"user_id"`
	Timestamp time.Time `json:"timestamp"`
	EventType string    `json:"event_type"`
}

type CacheService interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Get(ctx context.Context, key string, dest interface{}) error
	Delete(ctx context.Context, keys ...string) error
	RawClient() *redis.Client
}

type AuthService struct {
	userRepo    repository.UserRepository
	cfg         *configs.Config
	cache       CacheService
	mailService mail.Mailer
	sfGroup     singleflight.Group // Prevents cache stampede for concurrent requests
}

func NewAuthService(userRepo repository.UserRepository, cfg *configs.Config, cache CacheService, mailService mail.Mailer) *AuthService {
	return &AuthService{
		userRepo:    userRepo,
		cfg:         cfg,
		cache:       cache,
		mailService: mailService,
	}
}

func (s *AuthService) InitiateRegistration(ctx context.Context, input model.RegisterInput) (bool, error) {
	return s.userRepo.ExistsByEmail(ctx, input.Email)
}

func (s *AuthService) CreatePendingUser(ctx context.Context, user model.PendingUser) error {
	key := fmt.Sprintf("pending_user:%s", user.Email)
	expiration := 30 * 24 * time.Hour
	return s.cache.Set(ctx, key, user, expiration)
}

func (s *AuthService) GetPendingUser(ctx context.Context, email string) (*model.PendingUser, error) {
	key := fmt.Sprintf("pending_user:%s", email)
	var user model.PendingUser
	if err := s.cache.Get(ctx, key, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *AuthService) UpdatePendingUser(ctx context.Context, user model.PendingUser) error {
	key := fmt.Sprintf("pending_user:%s", user.Email)
	return s.cache.Set(ctx, key, user, time.Until(user.ExpiresAt))
}

func (s *AuthService) DeletePendingUser(ctx context.Context, email string) error {
	key := fmt.Sprintf("pending_user:%s", email)
	return s.cache.Delete(ctx, key)
}

func (s *AuthService) StoreVerificationDetails(ctx context.Context, email, code string) error {
	key := fmt.Sprintf("verification_code:%s", email)
	expiration := 5 * time.Minute
	err := s.cache.Set(ctx, key, code, expiration)

	if err != nil {
		return err
	}

	return nil
}

func (s *AuthService) CleanupTemporaryData(ctx context.Context, email string) error {
	codeKey := fmt.Sprintf("verification_code:%s", email)
	pendingUserKey := fmt.Sprintf("pending_user:%s", email)

	if err := s.cache.Delete(ctx, codeKey); err != nil {
		return err
	}
	return s.cache.Delete(ctx, pendingUserKey)
}

func (s *AuthService) ValidateVerificationCode(ctx context.Context, email, code string) (*model.PendingUser, error) {
	codeKey := fmt.Sprintf("verification_code:%s", email)
	pendingUserKey := fmt.Sprintf("pending_user:%s", email)

	var storedCode string
	err := s.cache.Get(ctx, codeKey, &storedCode)
	if err != nil {
		return nil, errors.OTPCodeExpire
	}
	if storedCode != code {
		return nil, errors.OTPCodeNotValid
	}

	var pendingUser *model.PendingUser

	err = s.cache.Get(ctx, pendingUserKey, &pendingUser)

	if err != nil {
		return nil, errors.UserNotFound
	}

	return pendingUser, nil
}

func (s *AuthService) VerifyCodeAndCreateUser(ctx context.Context, email, code string) (*ent.User, error) {

	pendingUser, err := s.GetPendingUser(ctx, email)
	if err != nil {
		return nil, errors.UserNotFound
	}

	if pendingUser.VerificationCode != code {
		return nil, errors.OTPCodeExpire
	}
	if time.Now().After(pendingUser.ExpiresAt) {
		return nil, errors.OTPCodeNotValid
	}

	user, err := s.userRepo.CreateNewUser(ctx, &model.RegisterVerifiedUser{
		Email:           pendingUser.Email,
		Password:        pendingUser.HashPassword,
		IsEmailVerified: true,
	})
	if err != nil {
		return nil, errors.NewTypedError("Something went wrong, Please try again", model.ErrorTypeInternalServerError, map[string]interface{}{"METHOD": "USER_CREATION"})
	}

	_ = s.CleanupTemporaryData(ctx, email)
	_ = s.DeletePendingUser(ctx, email)

	return user, nil
}

func (s *AuthService) InitiateLogin(ctx context.Context, email string) (*ent.User, error) {
	return s.userRepo.GetByEmail(ctx, email)
}

func (s *AuthService) FindUserProfileById(ctx context.Context, input int64) (*ent.User, error) {
	return s.userRepo.GetByID(ctx, input)
}

func (s *AuthService) UpdateLastLogin(ctx context.Context, userID int64) error {
	return s.userRepo.UpdateLoginTime(ctx, userID)
}

func (s *AuthService) PublishLoginEvent(ctx context.Context, userID int64) error {
	event := LoginEvent{
		UserID:    userID,
		Timestamp: time.Now(),
		EventType: "user_last_login",
	}

	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal login event: %w", err)
	}

	_, err = s.cache.RawClient().XAdd(ctx, &redis.XAddArgs{
		Stream: LoginStreamKey,
		MaxLen: 100000,
		Values: map[string]interface{}{"event": eventData},
	}).Result()

	return err
}

func (s *AuthService) BlacklistToken(ctx context.Context, token string, ttl time.Duration) error {
	key := fmt.Sprintf("blacklist:%s", token)
	return s.cache.Set(ctx, key, "blacklisted", ttl)
}

func (s *AuthService) IsTokenBlacklisted(ctx context.Context, token string) bool {
	var val string
	err := s.cache.Get(ctx, fmt.Sprintf("blacklist:%s", token), &val)
	return err == nil && val == "blacklisted"
}

func (s *AuthService) UpdateUserPassword(ctx context.Context, userID int64, passwordHash string) error {
	return s.userRepo.UpdateNewPassword(ctx, userID, passwordHash)
}

func (s *AuthService) StoreRefreshToken(ctx context.Context, userID int64, token string) (string, error) {

	encryptedToken, err := verification.EncryptToken(token)
	if err != nil {
		return "", err
	}

	hashedToken, err := verification.HashToken(token)
	if err != nil {
		return "", err
	}

	cacheKey := fmt.Sprintf("%s%d", RefreshCachePrefix, userID)
	err = s.cache.Set(ctx, cacheKey, encryptedToken, cookies.RefreshTokenExpiry)

	if err != nil {
		return "", err
	}

	hashKey := fmt.Sprintf("%s%s", cacheKey, ":hash")
	err = s.cache.Set(ctx, hashKey, hashedToken, cookies.RefreshTokenExpiry)

	if err != nil {
		return "", err
	}

	return hashedToken, nil
}

func (s *AuthService) ValidateRefreshToken(ctx context.Context, userID int64, token string) (bool, error) {
	cacheKey := fmt.Sprintf("%s%d", RefreshCachePrefix, userID)
	hashKey := fmt.Sprintf("%s%s", cacheKey, ":hash")

	var storedHash string
	err := s.cache.Get(ctx, hashKey, &storedHash)

	if err != nil {
		log.Printf("Error from stored hash %v", err)
		return false, err
	}

	valid, err := verification.VerifyTokenHash(token, storedHash)
	log.Printf("Error from VerifyTokenHash %v", err)
	if err != nil || !valid {
		return false, err
	}

	return true, nil
}

func (s *AuthService) InvalidateRefreshToken(ctx context.Context, userID int64) error {
	cacheKey := fmt.Sprintf("%s%d", RefreshCachePrefix, userID)
	hashKey := fmt.Sprintf("%s%s", cacheKey, ":hash")

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	return s.cache.Delete(ctx, cacheKey, hashKey)
}

func (s *AuthService) CheckIfRefreshTokenMatchClaims(ctx context.Context, uid int64) error {
	cacheKey := fmt.Sprintf("%s%d", RefreshCachePrefix, uid)
	var token string
	err := s.cache.Get(ctx, cacheKey, &token)

	if err != nil {
		return errors.ErrSomethingWentWrong
	}

	decryptedToken, err := verification.DecryptToken(token)
	if err != nil {
		return errors.ErrSomethingWentWrong
	}

	claims, err := jwt.ValidateToken(decryptedToken)
	if err != nil {
		return errors.InvalidToken
	}

	tokenUserID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		return errors.UserNotFound
	}

	if tokenUserID != uid {
		log.Printf("user ID mismatch: expected %d, got %d", uid, tokenUserID)
		errName := fmt.Sprintf("user ID mismatch: expected %d, got %d", uid, tokenUserID)
		return errors.NewTypedError(errName, model.ErrorTypeBadRequest, map[string]interface{}{
			"code": "BAD_REQUEST",
		})
	}

	return nil
}

func (s *AuthService) FindUsers(ctx context.Context, role *model.UserRole, pagination *model.PaginationInput) (*model.UserConnection, error) {
	return s.userRepo.FindAllUsers(ctx, role, pagination)
}

func (s *AuthService) CheckUsernameAvailability(ctx context.Context, username string) (bool, error) {
	cacheKey := fmt.Sprintf("username_exists:%s", username)
	var exists bool
	err := s.cache.Get(ctx, cacheKey, &exists)
	if err == nil {
		return !exists, nil
	}

	result, err, _ := s.sfGroup.Do(cacheKey, func() (interface{}, error) {
		// Check database
		exists, err := s.userRepo.ExistsByUsername(ctx, username)
		if err != nil {
			return false, err
		}

		_ = s.cache.Set(ctx, cacheKey, exists, 5*time.Minute)

		return !exists, nil
	})

	if err != nil {
		return false, err
	}

	return result.(bool), nil
}

func (s *AuthService) UpdateUsername(ctx context.Context, userID int64, newUsername string) error {

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	err = s.userRepo.UpdateUsername(ctx, userID, newUsername)
	if err != nil {
		return err
	}

	if user.Username != "" {
		oldCacheKey := fmt.Sprintf("username_exists:%s", user.Username)
		_ = s.cache.Delete(ctx, oldCacheKey)
	}

	newCacheKey := fmt.Sprintf("username_exists:%s", newUsername)
	_ = s.cache.Set(ctx, newCacheKey, true, 5*time.Minute)

	return nil
}
