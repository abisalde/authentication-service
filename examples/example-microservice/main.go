package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/abisalde/authentication-service/pkg/session"
)

func main() {
	// Load configuration from environment variables
	jwtSecret := os.Getenv("JWT_SECRET")
	redisAddr := os.Getenv("REDIS_ADDR")
	redisPassword := os.Getenv("REDIS_PASSWORD")
	port := os.Getenv("PORT")

	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable is required")
	}
	if redisAddr == "" {
		redisAddr = "localhost:6379" // Default
	}
	if port == "" {
		port = "8081"
	}

	// Initialize session validator
	validator, err := session.NewSessionValidator(session.Config{
		JWTSecret:     jwtSecret,
		RedisAddr:     redisAddr,
		RedisPassword: redisPassword,
		RedisDB:       0,
		Issuer:        "authentication-service",
		ClockSkew:     30 * time.Second,
	})
	if err != nil {
		log.Fatalf("Failed to initialize session validator: %v", err)
	}
	defer validator.Close()

	// Start listening for token invalidation events
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go validator.SubscribeToInvalidations(ctx)

	// Setup HTTP server with routes
	mux := http.NewServeMux()

	// Public endpoint - no authentication required
	mux.HandleFunc("/api/public", publicHandler)

	// Protected endpoint - authentication required
	mux.HandleFunc("/api/protected", session.RequireAuth(validator, protectedHandler))

	// User profile endpoint - authentication required
	mux.HandleFunc("/api/profile", session.RequireAuth(validator, profileHandler))

	// Health check endpoint
	mux.HandleFunc("/health", healthHandler)

	// Apply optional authentication middleware to all routes
	// This allows endpoints to check if user is authenticated
	handler := session.OptionalHTTPMiddleware(validator)(mux)

	// Setup HTTP server
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Starting example microservice on port %s", port)
		log.Printf("Public endpoint: http://localhost:%s/api/public", port)
		log.Printf("Protected endpoint: http://localhost:%s/api/protected", port)
		log.Printf("Profile endpoint: http://localhost:%s/api/profile", port)
		log.Printf("Health check: http://localhost:%s/health", port)
		
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Cancel context to stop invalidation subscription
	cancel()

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

// publicHandler handles public requests (no authentication)
func publicHandler(w http.ResponseWriter, r *http.Request) {
	// Check if user is authenticated (optional)
	userID, authenticated := session.GetUserID(r.Context())
	
	if authenticated {
		w.Write([]byte(fmt.Sprintf("Hello! You are authenticated as user: %s\n", userID)))
	} else {
		w.Write([]byte("Hello! This is a public endpoint. No authentication required.\n"))
	}
}

// protectedHandler handles protected requests (authentication required)
func protectedHandler(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context (guaranteed to exist because of RequireAuth)
	userID, _ := session.GetUserID(r.Context())
	tokenID, _ := session.GetTokenID(r.Context())
	
	response := fmt.Sprintf("Welcome authenticated user!\nUser ID: %s\nToken ID: %s\n", userID, tokenID)
	w.Write([]byte(response))
}

// profileHandler returns user profile information
func profileHandler(w http.ResponseWriter, r *http.Request) {
	// Get full claims from context
	claims, ok := session.GetClaims(r.Context())
	if !ok {
		http.Error(w, "Claims not found in context", http.StatusInternalServerError)
		return
	}

	// In a real application, you would fetch user data from your database
	response := fmt.Sprintf(`User Profile:
- User ID: %s
- Token ID: %s
- Issued At: %s
- Expires At: %s
- Issuer: %s
`, 
		claims.GetUserID(),
		claims.GetTokenID(),
		claims.IssuedAt.Time.Format(time.RFC3339),
		claims.ExpiresAt.Time.Format(time.RFC3339),
		claims.Issuer,
	)
	
	w.Write([]byte(response))
}

// healthHandler handles health check requests
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
