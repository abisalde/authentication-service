package auth

import (
	"context"
	"log"

	"github.com/abisalde/authentication-service/internal/database/ent"
	"github.com/gofiber/fiber/v2"
)

type contextKey string

var (
	CurrentUserKey        = contextKey("currentUser")
	ClientIPKey           = contextKey("clientIP")
	FiberContextWeb       = contextKey("fiberContextWebApplications")
	HTTPRequestKey        = contextKey("httpRequestForContext")
	HTTPResponseWriterKey = contextKey("httpResponseWriterForRequest")
	JWTTokenKey           = contextKey("JWTTokenKey")
	OAuthStateKey         = contextKey("serviceOAuthState")
	OAuthPlatformKey      = contextKey("serviceOAuthPlatform")
	OAuthModeKey          = contextKey("serviceOAuthPasswordLessMode")
	OAuthUUIDKey          = contextKey("serviceOAuthUUID")
	SessionInfoKey        = contextKey("sessionInfo")
)

func GetCurrentUser(ctx context.Context) *ent.User {
	if user, ok := ctx.Value(CurrentUserKey).(*ent.User); ok {
		return user
	}

	return nil
}

func GetIPFromContext(ctx context.Context) string {
	if ip, ok := ctx.Value(ClientIPKey).(string); ok {
		return ip
	}
	return ""
}

func GetFiberWebContext(ctx context.Context) (*fiber.Ctx, bool) {
	if fiberCtx, ok := ctx.Value(FiberContextWeb).(*fiber.Ctx); ok {
		return fiberCtx, true
	}
	return nil, false
}

func DebugContext(ctx context.Context) {
	log.Println("=== Context Debug START ===")
	if ip := GetIPFromContext(ctx); ip != "" {
		log.Printf("Auth Debug: Client IP in context: %s", ip)
	}

	if user := GetCurrentUser(ctx); user != nil {
		log.Printf("Auth Debug: User in context: ID=%d, Email=%s, Role=%s", user.ID, user.Email, user.Role)
	} else {
		log.Println("Auth Debug: No user in context.")
	}

	if v := ctx.Value(FiberContextWeb); v != nil {
		log.Printf("FiberWebKey: %v", v)
	} else {
		log.Println("FiberWebKey: nil")
	}

	log.Println("=========== Context Debug END =============")
}
