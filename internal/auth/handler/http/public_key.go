package http

import (
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
)

// PublicKeyHandler handles requests for the JWT public key
type PublicKeyHandler struct {
	jwtSecret    string
	algorithm    string
	publicKeyPath string
}

// NewPublicKeyHandler creates a new public key handler
func NewPublicKeyHandler() *PublicKeyHandler {
	algorithm := os.Getenv("JWT_ALGORITHM")
	if algorithm == "" {
		algorithm = "HS256" // Default to HS256 for backward compatibility
	}
	
	return &PublicKeyHandler{
		jwtSecret:    os.Getenv("JWT_SECRET"),
		algorithm:    algorithm,
		publicKeyPath: os.Getenv("JWT_PUBLIC_KEY_PATH"),
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
	var publicKey string
	var algorithm string
	
	// Determine algorithm and fetch appropriate key
	switch h.algorithm {
	case "RS256", "RS384", "RS512":
		// For RSA (RS256), read the actual public key file
		if h.publicKeyPath == "" {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "JWT_PUBLIC_KEY_PATH not configured for RS256",
			})
		}
		
		keyData, err := os.ReadFile(h.publicKeyPath)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to read public key file",
			})
		}
		
		publicKey = string(keyData)
		algorithm = h.algorithm
		
	case "HS256", "HS384", "HS512":
		// For HMAC (HS256), the "public key" is the shared secret
		// Note: This is less secure as the secret must be shared
		if h.jwtSecret == "" {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "JWT_SECRET not configured",
			})
		}
		
		publicKey = h.jwtSecret
		algorithm = h.algorithm
		
	default:
		// Default to HS256 for backward compatibility
		if h.jwtSecret == "" {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "JWT_SECRET not configured",
			})
		}
		
		publicKey = h.jwtSecret
		algorithm = "HS256"
	}

	response := PublicKeyResponse{
		PublicKey: publicKey,
		Algorithm: algorithm,
		KeyID:     "default",
		ExpiresAt: time.Now().Add(5 * time.Minute), // Cache for 5 minutes
		Version:   "1.0",
	}

	// Set cache headers for microservices to cache the response
	c.Set("Cache-Control", "public, max-age=300") // 5 minutes
	c.Set("X-Key-Version", response.Version)
	c.Set("X-Key-Algorithm", algorithm)

	return c.JSON(response)
}

// RegisterRoutes registers the public key routes
func (h *PublicKeyHandler) RegisterRoutes(app *fiber.App) {
	app.Get("/api/public-key", h.GetPublicKey)
}
