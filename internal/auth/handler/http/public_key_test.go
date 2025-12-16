package http

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublicKeyHandler_GetPublicKey_HS256(t *testing.T) {
	tests := []struct {
		name           string
		jwtSecret      string
		jwtAlgorithm   string
		expectedStatus int
		checkResponse  func(t *testing.T, resp *PublicKeyResponse)
	}{
		{
			name:           "Success - Returns public key with HS256",
			jwtSecret:      "test-secret-key-12345",
			jwtAlgorithm:   "HS256",
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
			name:           "Success - Defaults to HS256 when no algorithm specified",
			jwtSecret:      "test-secret-key",
			jwtAlgorithm:   "",
			expectedStatus: fiber.StatusOK,
			checkResponse: func(t *testing.T, resp *PublicKeyResponse) {
				assert.Equal(t, "test-secret-key", resp.PublicKey)
				assert.Equal(t, "HS256", resp.Algorithm)
			},
		},
		{
			name:           "Error - JWT_SECRET not configured",
			jwtSecret:      "",
			jwtAlgorithm:   "HS256",
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
			if tt.jwtAlgorithm != "" {
				os.Setenv("JWT_ALGORITHM", tt.jwtAlgorithm)
			} else {
				os.Unsetenv("JWT_ALGORITHM")
			}
			defer os.Unsetenv("JWT_SECRET")
			defer os.Unsetenv("JWT_ALGORITHM")

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
				assert.NotEmpty(t, resp.Header.Get("X-Key-Algorithm"))
			}
		})
	}
}

func TestPublicKeyHandler_GetPublicKey_RS256(t *testing.T) {
	// Create temporary public key file
	tmpDir := t.TempDir()
	publicKeyPath := filepath.Join(tmpDir, "test_public.pem")
	testPublicKey := `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAtest
-----END PUBLIC KEY-----`
	
	err := os.WriteFile(publicKeyPath, []byte(testPublicKey), 0644)
	require.NoError(t, err)

	tests := []struct {
		name           string
		publicKeyPath  string
		jwtAlgorithm   string
		expectedStatus int
		checkResponse  func(t *testing.T, resp *PublicKeyResponse)
	}{
		{
			name:           "Success - Returns RSA public key",
			publicKeyPath:  publicKeyPath,
			jwtAlgorithm:   "RS256",
			expectedStatus: fiber.StatusOK,
			checkResponse: func(t *testing.T, resp *PublicKeyResponse) {
				assert.Equal(t, testPublicKey, resp.PublicKey)
				assert.Equal(t, "RS256", resp.Algorithm)
				assert.Equal(t, "default", resp.KeyID)
				assert.Equal(t, "1.0", resp.Version)
				assert.True(t, resp.ExpiresAt.After(time.Now()))
			},
		},
		{
			name:           "Error - JWT_PUBLIC_KEY_PATH not configured",
			publicKeyPath:  "",
			jwtAlgorithm:   "RS256",
			expectedStatus: fiber.StatusInternalServerError,
			checkResponse:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			os.Setenv("JWT_ALGORITHM", tt.jwtAlgorithm)
			if tt.publicKeyPath != "" {
				os.Setenv("JWT_PUBLIC_KEY_PATH", tt.publicKeyPath)
			} else {
				os.Unsetenv("JWT_PUBLIC_KEY_PATH")
			}
			defer os.Unsetenv("JWT_ALGORITHM")
			defer os.Unsetenv("JWT_PUBLIC_KEY_PATH")

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
				assert.Equal(t, "RS256", resp.Header.Get("X-Key-Algorithm"))
			}
		})
	}
}

func TestPublicKeyHandler_CacheHeaders(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret")
	os.Setenv("JWT_ALGORITHM", "HS256")
	defer os.Unsetenv("JWT_SECRET")
	defer os.Unsetenv("JWT_ALGORITHM")

	handler := NewPublicKeyHandler()
	app := fiber.New()
	handler.RegisterRoutes(app)

	req := httptest.NewRequest("GET", "/api/public-key", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify cache headers are set correctly
	assert.Equal(t, "public, max-age=300", resp.Header.Get("Cache-Control"))
	assert.Equal(t, "1.0", resp.Header.Get("X-Key-Version"))
	assert.Equal(t, "HS256", resp.Header.Get("X-Key-Algorithm"))
}
