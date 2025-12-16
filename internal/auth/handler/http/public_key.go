package http

import (
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
)

// PublicKeyHandler handles requests for the JWT public key
type PublicKeyHandler struct {
	jwtSecret string
}

// NewPublicKeyHandler creates a new public key handler
func NewPublicKeyHandler() *PublicKeyHandler {
	return &PublicKeyHandler{
		jwtSecret: os.Getenv("JWT_SECRET"),
	}
}

// PublicKeyResponse represents the public key API response
type PublicKeyResponse struct {
	PublicKey string    `json:"publicKey"`
	Algorithm string    `json:"algorithm"`
	KeyID     string    `json:"keyId"`
	ExpiresAt time.Time `json:"expiresAt"`
	Version   string    `json:"version"`
}

// GetPublicKey returns the JWT public key for token validation
// @Summary Get JWT public key
// @Description Returns the public key used for JWT token validation. Microservices should cache this key.
// @Tags Authentication
// @Produce json
// @Success 200 {object} PublicKeyResponse
// @Failure 500 {object} map[string]interface{}
// @Router /api/public-key [get]
func (h *PublicKeyHandler) GetPublicKey(c *fiber.Ctx) error {
	if h.jwtSecret == "" {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "JWT_SECRET not configured",
		})
	}

	// For HMAC-based JWT (HS256), the "public key" is the shared secret
	// In production, consider migrating to RS256 with actual public/private key pairs
	response := PublicKeyResponse{
		PublicKey: h.jwtSecret,
		Algorithm: "HS256",
		KeyID:     "default",
		ExpiresAt: time.Now().Add(5 * time.Minute), // Cache for 5 minutes
		Version:   "1.0",
	}

	// Set cache headers for microservices to cache the response
	c.Set("Cache-Control", "public, max-age=300") // 5 minutes
	c.Set("X-Key-Version", response.Version)

	return c.JSON(response)
}

// RegisterRoutes registers the public key routes
func (h *PublicKeyHandler) RegisterRoutes(app *fiber.App) {
	app.Get("/api/public-key", h.GetPublicKey)
}
