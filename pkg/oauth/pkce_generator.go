package oauthPKCE

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

const (
	codeVerifierLength = 32
)

func GeneratePKCEVerifier() (string, error) {
	randomBytes := make([]byte, codeVerifierLength)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(randomBytes), nil
}

func GeneratePKCEChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}
