package http

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublicKeyHandler_GetPublicKey(t *testing.T) {
	tests := []struct {
		name           string
		jwtSecret      string
		expectedStatus int
		checkResponse  func(t *testing.T, resp *PublicKeyResponse)
	}{
		{
			name:           "Success - Returns public key",
			jwtSecret:      "test-secret-key-12345",
			expectedStatus: fiber.StatusOK,
			checkResponse: func(t *testing.T, resp *PublicKeyResponse) {
				assert.Equal(t, "test-secret-key-12345", resp.PublicKey)
				assert.Equal(t, "HS256", resp.Algorithm)
				assert.Equal(t, "default", resp.KeyID)
				assert.Equal(t, "1.0", resp.Version)
				
				// Check that expiresAt is in the future (within 6 minutes)
				assert.True(t, resp.ExpiresAt.After(time.Now()))
				assert.True(t, resp.ExpiresAt.Before(time.Now().Add(6*time.Minute)))
			},
		},
		{
			name:           "Error - JWT_SECRET not configured",
			jwtSecret:      "",
			expectedStatus: fiber.StatusInternalServerError,
			checkResponse:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			if tt.jwtSecret != "" {
				os.Setenv("JWT_SECRET", tt.jwtSecret)
			} else {
				os.Unsetenv("JWT_SECRET")
			}
			defer os.Unsetenv("JWT_SECRET")

			// Create handler and app
			handler := NewPublicKeyHandler()
			app := fiber.New()
			handler.RegisterRoutes(app)

			// Create test request
			req := httptest.NewRequest("GET", "/api/public-key", nil)
			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Check status code
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			// Check response body
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			if tt.checkResponse != nil {
				var publicKeyResp PublicKeyResponse
				err = json.Unmarshal(body, &publicKeyResp)
				require.NoError(t, err)

				tt.checkResponse(t, &publicKeyResp)

				// Check cache headers
				assert.Equal(t, "public, max-age=300", resp.Header.Get("Cache-Control"))
				assert.Equal(t, "1.0", resp.Header.Get("X-Key-Version"))
			}
		})
	}
}

func TestPublicKeyHandler_CacheHeaders(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret")
	defer os.Unsetenv("JWT_SECRET")

	handler := NewPublicKeyHandler()
	app := fiber.New()
	handler.RegisterRoutes(app)

	req := httptest.NewRequest("GET", "/api/public-key", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify cache headers are set correctly
	assert.Equal(t, "public, max-age=300", resp.Header.Get("Cache-Control"))
	assert.NotEmpty(t, resp.Header.Get("X-Key-Version"))
}
