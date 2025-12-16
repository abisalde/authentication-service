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
	"github.com/abisalde/authentication-service/pkg/jwt"
	"github.com/abisalde/authentication-service/pkg/password"
	"github.com/abisalde/authentication-service/pkg/session"
	"github.com/gofiber/fiber/v2"
)

type LoginHandler struct {
	authService    *service.AuthService
	sessionManager *session.SessionManager
}

func NewLoginHandler(authService *service.AuthService) *LoginHandler {
	return &LoginHandler{
		authService:    authService,
		sessionManager: session.NewSessionManager(authService.GetCache().RawClient()),
	}
}

func (h *LoginHandler) EmailLogin(ctx context.Context, input model.LoginInput) (*model.LoginResponse, error) {

	user, err := h.authService.InitiateLogin(ctx, input.Email)
	if err != nil {
		return nil, errors.InvalidCredentialsEmail
	}

	err = password.CheckPasswordHash(input.Password, user.PasswordHash)
	if err != nil {
		return nil, errors.InvalidCredentialsPassword
	}

	tokens, err := cookies.GenerateLoginTokenPair(user.ID)

	if err != nil {
		log.Printf("This is error from cookies.GenerateLoginTokenPair: %v", err)
		return nil, errors.ErrSomethingWentWrong
	}

	// Store and Hash the RefreshToken
	hashedToken, refreshErr := h.authService.StoreRefreshToken(ctx, user.ID, tokens.RefreshToken)

	if refreshErr != nil {
		return nil, errors.ErrSomethingWentWrong
	}

	if fiberCtx, ok := ctx.Value(auth.FiberContextWeb).(*fiber.Ctx); ok {
		err = cookies.CreateBrowserSession(cookies.TokenPair{
			AccessToken:  tokens.AccessToken,
			RefreshToken: hashedToken,
		}, fiberCtx)
		if err != nil {
			return nil, errors.ErrSomethingWentWrong
		}
	}

	err = h.authService.UpdateLastLogin(ctx, user.ID)
	if err != nil {
		return nil, errors.ErrSomethingWentWrong
	}

	// Create session for multi-device tracking
	if err := h.createUserSession(ctx, user.ID, tokens.AccessToken); err != nil {
		log.Printf("Failed to create session: %v", err)
		// Don't fail login if session creation fails
	}

	return &model.LoginResponse{
		UserId:       user.ID,
		Token:        tokens.AccessToken,
		RefreshToken: hashedToken,
		Email:        user.Email,
	}, nil
}

// createUserSession creates a session for device tracking
func (h *LoginHandler) createUserSession(ctx context.Context, userID int64, accessToken string) error {
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

	sessionInfo := &session.SessionInfo{
		UserID:     strconv.FormatInt(userID, 10),
		DeviceType: deviceInfo.Type,
		DeviceName: deviceInfo.Name,
		IPAddress:  deviceInfo.IPAddress,
		UserAgent:  deviceInfo.UserAgent,
		TokenHash:  session.HashToken(accessToken),
		CreatedAt:  time.Now(),
		LastUsedAt: time.Now(),
		ExpiresAt:  time.Now().Add(cookies.LoginAccessTokenExpiry),
	}

	// Optional: Enforce max 10 concurrent sessions per user
	if err := h.sessionManager.EnforceMaxSessions(ctx, sessionInfo.UserID, 10); err != nil {
		log.Printf("Failed to enforce max sessions: %v", err)
	}

	return h.sessionManager.CreateSession(ctx, sessionInfo)
}

func (h *LoginHandler) ProcessLogout(ctx context.Context) (bool, error) {
	currentUser := auth.GetCurrentUser(ctx)
	if currentUser == nil {
		return false, errors.AuthenticationRequired
	}

	_ = h.authService.InvalidateRefreshToken(ctx, currentUser.ID)

	token, ok := ctx.Value(auth.JWTTokenKey).(string)
	if ok && token != "" {
		remainingTTL := jwt.GetTokenRemainingTTL(token)
		if remainingTTL > 0 {
			h.authService.BlacklistToken(ctx, token, remainingTTL)
		}

		// Revoke the current session
		tokenHash := session.HashToken(token)
		userIDStr := strconv.FormatInt(currentUser.ID, 10)
		if sess, err := h.sessionManager.GetSessionByTokenHash(ctx, userIDStr, tokenHash); err == nil {
			if err := h.sessionManager.RevokeSession(ctx, userIDStr, sess.SessionID); err != nil {
				log.Printf("Failed to revoke session on logout: %v", err)
			}
		}
	}

	if fiberCtx, ok := ctx.Value(auth.FiberContextWeb).(*fiber.Ctx); ok {
		fiberCtx.ClearCookie(cookies.BrowserAccessTokenName)
		fiberCtx.ClearCookie(cookies.BrowserSessionTokenName)
	}
	if w, ok := ctx.Value(auth.HTTPResponseWriterKey).(http.ResponseWriter); ok {
		http.SetCookie(w, &http.Cookie{Name: cookies.BrowserAccessTokenName, MaxAge: -1, Path: "/"})
		http.SetCookie(w, &http.Cookie{Name: cookies.BrowserSessionTokenName, MaxAge: -1, Path: "/"})
	}

	return true, nil
}
