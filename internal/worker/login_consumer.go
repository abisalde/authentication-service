package worker

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/abisalde/authentication-service/internal/auth/service"
	"github.com/redis/go-redis/v9"
)

type LastLoginWorker struct {
	redisClient *redis.Client
	authService service.AuthService
}

func NewLastLoginWorker(redisClient *redis.Client, authService service.AuthService) *LastLoginWorker {
	return &LastLoginWorker{
		redisClient: redisClient,
		authService: authService,
	}
}

func (w *LastLoginWorker) Start(ctx context.Context) {
	lastID := "0"

	for {

		select {
		case <-ctx.Done():
			log.Println("LoginEventConsumer shutting down.")
			return
		default:
			streams, err := w.redisClient.XRead(ctx, &redis.XReadArgs{
				Streams: []string{service.LoginStreamKey, lastID},
				Block:   0,
			}).Result()

			if err != nil {
				log.Printf("Error reading from stream: %v", err)
				time.Sleep(1 * time.Second)
				continue
			}

			for _, stream := range streams {
				for _, msg := range stream.Messages {

					eventString := msg.Values["event"].(string)

					var loginEvent service.LoginEvent
					if err := json.Unmarshal([]byte(eventString), &loginEvent); err != nil {
						log.Printf("Failed to unmarshal event: %v", err)
						continue
					}
					if loginEvent.EventType == "user_last_login" {
						err := w.authService.UpdateLastLogin(ctx, loginEvent.UserID)
						if err != nil {
							log.Printf("Failed to update last login for user %v: %v", loginEvent.UserID, err)
						}
					}
					lastID = msg.ID
				}
			}
		}

	}
}
