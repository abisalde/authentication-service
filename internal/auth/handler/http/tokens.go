package http

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/abisalde/authentication-service/internal/auth"
	"github.com/abisalde/authentication-service/internal/auth/cookies"
	"github.com/abisalde/authentication-service/internal/auth/service"
	"github.com/abisalde/authentication-service/internal/graph/errors"
	"github.com/abisalde/authentication-service/internal/graph/model"
	"github.com/abisalde/authentication-service/pkg/session"
	"github.com/gofiber/fiber/v2"
)

type TokenHandler struct {
	authService    *service.AuthService
	sessionManager *session.SessionManager
}

func NewTokenHandler(authService *service.AuthService) *TokenHandler {
	return &TokenHandler{
		authService:    authService,
		sessionManager: session.NewSessionManager(authService.GetCache().RawClient()),
	}
}

func (h *TokenHandler) HandleRefreshToken(
	ctx context.Context, token string, uid int32,
) (*model.RefreshTokenResponse, error) {

	userID := int64(uid)

	ok, err := h.authService.ValidateRefreshToken(ctx, userID, token)
	if !ok {
		log.Printf("Error from validating refresh token: %v", err)
		return nil, errors.InvalidRefreshTokenValidation
	}

	err = h.authService.CheckIfRefreshTokenMatchClaims(ctx, userID)
	log.Printf("h.authService.CheckIfRefreshTokenMatchClaims %v", err)
	if err != nil {
		return nil, err
	}

	accessToken, err := cookies.GenerateAccessToken(userID)
	if err != nil {
		log.Printf("Error from generating access token: %v", err)
		return nil, errors.AccessTokenGeneration
	}

	// Create or update session for the new access token
	if err := h.updateSessionForRefreshToken(ctx, userID, accessToken); err != nil {
		log.Printf("Failed to update session after token refresh: %v", err)
		// Don't fail token refresh if session update fails
	}

	return &model.RefreshTokenResponse{
		Token: accessToken,
	}, nil
}

// updateSessionForRefreshToken creates or updates session when a refresh token is used
func (h *TokenHandler) updateSessionForRefreshToken(ctx context.Context, userID int64, accessToken string) error {
	// Extract device info from context
	var deviceInfo *session.DeviceInfo
	
	// Try to get HTTP request from context
	if req, ok := ctx.Value(auth.FiberContextWeb).(*http.Request); ok {
		deviceInfo = session.ExtractDeviceInfo(req)
	} else if fiberCtx, ok := ctx.Value(auth.FiberContextWeb).(*fiber.Ctx); ok {
		// Convert fiber.Ctx to http.Request-like structure
		req := &http.Request{
			Header:     make(http.Header),
			RemoteAddr: fiberCtx.IP(),
		}
		// Copy headers
		fiberCtx.Request().Header.VisitAll(func(key, value []byte) {
			req.Header.Add(string(key), string(value))
		})
		deviceInfo = session.ExtractDeviceInfo(req)
	} else {
		// Fallback to minimal device info
		deviceInfo = &session.DeviceInfo{
			Type:      "Unknown",
			Name:      "Unknown",
			IPAddress: auth.GetIPFromContext(ctx),
			UserAgent: "Unknown",
		}
	}

	userIDStr := strconv.FormatInt(userID, 10)
	tokenHash := session.HashToken(accessToken)

	// Check if a session already exists for this device
	existingSessions, err := h.sessionManager.GetUserSessions(ctx, userIDStr)
	if err == nil {
		// Look for a session with matching device info (same device, different token)
		for _, sess := range existingSessions {
			if sess.DeviceType == deviceInfo.Type && 
			   sess.DeviceName == deviceInfo.Name && 
			   sess.IPAddress == deviceInfo.IPAddress {
				// Update existing session with new token hash
				sess.TokenHash = tokenHash
				sess.LastUsedAt = time.Now()
				sess.ExpiresAt = time.Now().Add(cookies.LoginAccessTokenExpiry)
				
				// Delete old session and create new one
				h.sessionManager.RevokeSession(ctx, userIDStr, sess.SessionID)
				break
			}
		}
	}

	// Create new session for refreshed token
	sessionInfo := &session.SessionInfo{
		UserID:     userIDStr,
		DeviceType: deviceInfo.Type,
		DeviceName: deviceInfo.Name,
		IPAddress:  deviceInfo.IPAddress,
		UserAgent:  deviceInfo.UserAgent,
		TokenHash:  tokenHash,
		CreatedAt:  time.Now(),
		LastUsedAt: time.Now(),
		ExpiresAt:  time.Now().Add(cookies.LoginAccessTokenExpiry),
	}

	// Enforce max sessions
	if err := h.sessionManager.EnforceMaxSessions(ctx, userIDStr, 10); err != nil {
		log.Printf("Failed to enforce max sessions: %v", err)
	}

	return h.sessionManager.CreateSession(ctx, sessionInfo)
}
