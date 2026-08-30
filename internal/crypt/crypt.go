package crypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/argon2"
)

const (
	Magic        = "CUENCv1\n"
	saltSize     = 16
	nonceSize    = 12
	keySize      = 32
	argonTime    = 3
	argonMem     = 64 * 1024
	argonThreads = 4
)

var (
	ErrNotEncrypted  = errors.New("payload is not encrypted")
	ErrWrongPassword = errors.New("wrong password")
	ErrCorrupt       = errors.New("encrypted payload is corrupt")
)

// IsEncrypted reports whether data starts with the Captain Updater magic header.
func IsEncrypted(data []byte) bool {
	return len(data) >= len(Magic) && string(data[:len(Magic)]) == Magic
}

func deriveKey(password string, salt []byte) []byte {
	return argon2.IDKey([]byte(password), salt, argonTime, argonMem, argonThreads, keySize)
}

// Seal encrypts plaintext with a password.
func Seal(plaintext []byte, password string) ([]byte, error) {
	if password == "" {
		return nil, errors.New("password required")
	}

	salt := make([]byte, saltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("salt: %w", err)
	}

	block, err := aes.NewCipher(deriveKey(password, salt))
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}

	sealed := gcm.Seal(nil, nonce, plaintext, nil)
	out := make([]byte, 0, len(Magic)+saltSize+nonceSize+len(sealed))
	out = append(out, Magic...)
	out = append(out, salt...)
	out = append(out, nonce...)
	out = append(out, sealed...)

	return out, nil
}

// Open decrypts a sealed blob. Returns ErrNotEncrypted if the magic is missing.
func Open(blob []byte, password string) ([]byte, error) {
	if !IsEncrypted(blob) {
		return nil, ErrNotEncrypted
	}

	rest := blob[len(Magic):]
	if len(rest) < saltSize+nonceSize+16 {
		return nil, ErrCorrupt
	}

	salt := rest[:saltSize]
	nonce := rest[saltSize : saltSize+nonceSize]
	sealed := rest[saltSize+nonceSize:]

	block, err := aes.NewCipher(deriveKey(password, salt))
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	plain, err := gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return nil, ErrWrongPassword
	}

	return plain, nil
}
