package services

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"os"
)

// encryptionKey derives a 32-byte AES-256 key from ENCRYPTION_KEY, so any
// non-empty env value produces a valid key regardless of its length -
// mirroring auth_service.go's JWT_SECRET fallback pattern (falls back to
// a documented dev-only default rather than failing startup).
func encryptionKey() [32]byte {
	key := os.Getenv("ENCRYPTION_KEY")
	if key == "" {
		key = "default-encryption-key-change-in-production"
	}
	return sha256.Sum256([]byte(key))
}

// Encrypt returns base64(nonce || ciphertext) for plaintext, using
// AES-256-GCM with a fresh random nonce per call.
func Encrypt(plaintext string) (string, error) {
	key := encryptionKey()
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt reverses Encrypt. It returns an error if ciphertext is
// malformed or its authentication tag doesn't verify (wrong key or
// tampered data).
func Decrypt(ciphertext string) (string, error) {
	key := encryptionKey()
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(raw) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	nonce, sealed := raw[:nonceSize], raw[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
