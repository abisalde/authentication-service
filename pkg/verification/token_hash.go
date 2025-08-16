package verification

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

var (
	secretOnce sync.Once
	loadError  error

	refreshTokenHash []byte
	encSecret        []byte

	ErrInvalidToken         = errors.New("invalid token")
	ErrDecryption           = errors.New("decryption failed")
	ErrEncryptionSecretSize = errors.New("invalid encryption secret size")
)

func loadSecret() error {
	secretOnce.Do(func() {
		refreshTokenHashVal := os.Getenv("REFRESH_TOKEN_HASH_SECRET")
		encSecretVal := os.Getenv("REFRESH_TOKEN_ENC_SECRET")

		if refreshTokenHashVal == "" || encSecretVal == "" {
			loadError = errors.New("token environment not configured")
			return
		}
		refreshTokenHash = []byte(refreshTokenHashVal)
		decodedEncSecret, err := base64.StdEncoding.DecodeString(encSecretVal)
		if err != nil {
			loadError = fmt.Errorf("failed to decode encryption secret: %w", err)
			return
		}
		encSecret = decodedEncSecret
	})
	return loadError
}

func HashToken(token string) (string, error) {
	if err := loadSecret(); err != nil {
		return "", err
	}

	h := hmac.New(sha256.New, refreshTokenHash)
	h.Write([]byte(token))
	return base64.URLEncoding.EncodeToString(h.Sum(nil)), nil
}

func VerifyTokenHash(token, storedHash string) (bool, error) {
	return subtle.ConstantTimeCompare([]byte(token), []byte(storedHash)) == 1, nil
}

func EncryptToken(token string) (string, error) {
	if err := loadSecret(); err != nil {
		return "", err
	}

	if len(encSecret) != 32 {
		return "", errors.New("invalid encryption secret size")
	}

	block, err := aes.NewCipher(encSecret)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(token), nil)
	return base64.URLEncoding.EncodeToString(ciphertext), nil
}

func DecryptToken(encryptedToken string) (string, error) {
	if err := loadSecret(); err != nil {
		return "", err
	}
	if len(encSecret) != 32 {
		return "", ErrEncryptionSecretSize
	}

	ciphertext, err := base64.URLEncoding.DecodeString(encryptedToken)
	if err != nil {
		return "", fmt.Errorf("base64 decode failed: %w", err)
	}

	block, err := aes.NewCipher(encSecret)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	if len(ciphertext) < gcm.NonceSize() {
		return "", ErrDecryption
	}

	nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrDecryption
	}

	return string(plaintext), nil
}
