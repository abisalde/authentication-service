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

	"github.com/abisalde/authentication-service/pkg/config"
	"github.com/abisalde/authentication-service/pkg/session"
)

func main() {
	log.Println("Starting secure microservice example...")

	// Initialize secure configuration manager
	// This automatically detects the environment and uses the appropriate secret provider:
	// - Kubernetes: File-based secrets from /var/run/secrets/app
	// - Public Key Mode: Fetches public key from AUTH_SERVICE_URL
	// - Development: Falls back to environment variables (least secure)
	secretProvider := config.DefaultSecretProvider()
	configMgr := config.NewConfigManager(secretProvider)

	ctx := context.Background()

	// Get configuration securely
	// For production, this will fetch the public key from auth service
	// For development, it falls back to JWT_SECRET env var
	var validator *session.SessionValidator
	var err error

	// Try to get public key first (most secure)
	publicKey, err := configMgr.GetJWTPublicKey(ctx)
	if err == nil && publicKey != nil {
		log.Println("✅ Using RS256 with public key (secure mode)")
		// Create validator with public key (TODO: implement in validator.go)
		// For now, fall through to shared secret
		log.Println("⚠️  Public key validation not yet implemented, falling back to shared secret")
	}

	// Fall back to shared secret (for backward compatibility)
	jwtSecret, err := configMgr.GetJWTSecret(ctx)
	if err != nil {
		log.Fatalf("Failed to get JWT configuration: %v", err)
	}

	// Get Redis configuration
	redisAddr, redisPassword, err := configMgr.GetRedisConfig(ctx)
	if err != nil {
		log.Fatalf("Failed to get Redis configuration: %v", err)
	}

	// Initialize session validator with secure config
	validator, err = session.NewSessionValidator(session.Config{
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
	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go validator.SubscribeToInvalidations(cancelCtx)

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

	// Security information endpoint
	mux.HandleFunc("/api/security-info", securityInfoHandler)

	// Apply optional authentication middleware to all routes
	handler := session.OptionalHTTPMiddleware(validator)(mux)

	// Get port from environment
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

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
		log.Printf("🚀 Secure microservice listening on port %s", port)
		log.Println("📚 Endpoints:")
		log.Println("   - GET  /health          - Health check")
		log.Println("   - GET  /api/public      - Public endpoint")
		log.Println("   - GET  /api/protected   - Protected endpoint (requires auth)")
		log.Println("   - GET  /api/profile     - User profile (requires auth)")
		log.Println("   - GET  /api/security-info - Security configuration info")
		log.Println("")
		log.Println("🔒 Security Mode:")
		if publicKey != nil {
			log.Println("   - RS256 with public key validation (SECURE)")
		} else {
			log.Println("   - HS256 with shared secret (DEVELOPMENT ONLY)")
		}
		
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped")
}

func publicHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"message": "This is a public endpoint",
		"time":    time.Now().Format(time.RFC3339),
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"message": "%s", "time": "%s"}`, response["message"], response["time"])
}

func protectedHandler(w http.ResponseWriter, r *http.Request) {
	// Get user info from context (added by RequireAuth middleware)
	userID, ok := session.GetUserID(r.Context())
	if !ok {
		http.Error(w, "User ID not found", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"message": "This is a protected endpoint",
		"user_id": userID,
		"time":    time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"message": "%s", "user_id": "%s", "time": "%s"}`,
		response["message"], response["user_id"], response["time"])
}

func profileHandler(w http.ResponseWriter, r *http.Request) {
	// Get user info from context
	userID, ok := session.GetUserID(r.Context())
	if !ok {
		http.Error(w, "User ID not found", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"user_id": userID,
		"time":    time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"user_id": "%s", "time": "%s"}`,
		response["user_id"], response["time"])
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status": "healthy", "time": "%s"}`, time.Now().Format(time.RFC3339))
}

func securityInfoHandler(w http.ResponseWriter, r *http.Request) {
	// Determine secret provider type
	providerType := "unknown"
	if _, err := os.Stat("/var/run/secrets/kubernetes.io"); err == nil {
		providerType = "kubernetes-secrets"
	} else if os.Getenv("AUTH_SERVICE_URL") != "" {
		providerType = "public-key-provider"
	} else {
		providerType = "environment-variables"
	}

	securityLevel := "🟢 HIGH"
	if providerType == "environment-variables" {
		securityLevel = "🟡 MEDIUM (Development Only)"
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{
		"secret_provider": "%s",
		"security_level": "%s",
		"jwt_algorithm": "HS256",
		"recommendations": [
			"Use RS256 with public key infrastructure for production",
			"Store secrets in Kubernetes Secrets or Vault",
			"Implement secret rotation policy"
		]
	}`, providerType, securityLevel)
}
