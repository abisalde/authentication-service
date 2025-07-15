package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/abisalde/authentication-service/internal/auth/repository"
	"github.com/abisalde/authentication-service/internal/configs"
	"github.com/abisalde/authentication-service/internal/database/ent"
	"github.com/abisalde/authentication-service/internal/graph/errors"
	"github.com/abisalde/authentication-service/internal/graph/model"
	"github.com/abisalde/authentication-service/pkg/mail"
	"github.com/redis/go-redis/v9"
)

const (
	LoginStreamKey = "login_events"
	LoginGroup     = "login_event_group"
)

type LoginEvent struct {
	UserID    int64     `json:"user_id"`
	Timestamp time.Time `json:"timestamp"`
	EventType string    `json:"event_type"`
}

type CacheService interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Get(ctx context.Context, key string, dest interface{}) error
	Delete(ctx context.Context, key string) error
	RawClient() *redis.Client
}

type AuthService struct {
	userRepo    repository.UserRepository
	cfg         *configs.Config
	cache       CacheService
	mailService *mail.SMTPMailService
}

func NewAuthService(userRepo repository.UserRepository, cfg *configs.Config, cache CacheService, mailService *mail.SMTPMailService) *AuthService {
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

	// XADD command: Add message to stream, '*' generates a unique ID
	_, err = s.cache.RawClient().XAdd(ctx, &redis.XAddArgs{
		Stream: LoginStreamKey,
		MaxLen: 100000,
		Values: map[string]interface{}{"event": eventData},
	}).Result()

	// if cmd.Err() != nil {
	// 	return fmt.Errorf("failed to publish login event to Redis Stream: %w", cmd.Err())
	// }

	// log.Printf("Published login event for UserID: %d to stream %s with ID %s\n", userID, LoginStreamKey, cmd.Val())
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
