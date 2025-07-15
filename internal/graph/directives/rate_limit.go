package directives

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/abisalde/authentication-service/internal/auth"
	"github.com/abisalde/authentication-service/internal/database"
	"github.com/abisalde/authentication-service/internal/database/ent"
	"github.com/abisalde/authentication-service/internal/graph/errors"
	"github.com/abisalde/authentication-service/internal/graph/model"
)

type RateLimitDirective struct {
	redisCache *database.RedisCache
}

func NewRateLimitDirective(redisCache *database.RedisCache) *RateLimitDirective {
	return &RateLimitDirective{
		redisCache: redisCache,
	}
}

func (r *RateLimitDirective) RateLimit(
	ctx context.Context,
	obj interface{},
	next graphql.Resolver,
	operation model.RateLimitMethods,
	limit int32,
	duration int32,
) (interface{}, error) {

	auth.DebugContext(ctx)

	if r.redisCache == nil {
		log.Println("redisCache is nil")
		return nil, fmt.Errorf("rate limiter not initialized")
	}

	user := auth.GetCurrentUser(ctx)
	ip := auth.GetIPFromContext(ctx)

	identifier := r.getIdentifier(user, ip)

	window := time.Duration(duration) * time.Second
	expiration := time.Now().Unix() / int64(window.Seconds())
	windowKey := fmt.Sprintf("rate_limit:%s:%s:%d", operation.String(), identifier, expiration)

	pipe := r.redisCache.RawClient().TxPipeline()
	incr := pipe.Incr(ctx, windowKey)
	pipe.Expire(ctx, windowKey, window)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return nil, errors.RateLimitExceeded
	}

	count := incr.Val()
	if count > int64(limit) {
		return nil, errors.RateLimitExceeded
	}

	return next(ctx)

}

func (r *RateLimitDirective) getIdentifier(user *ent.User, ip string) string {
	switch {
	case user != nil:
		return fmt.Sprintf("user:%v", user.ID)
	case ip != "":
		return fmt.Sprintf("ip:%s", ip)
	default:
		return "anonymous"
	}
}
