// Package crypto provides AES-256-GCM authenticated encryption.
// No KMS or external key management is used; all keys are caller-supplied.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
)

// KeySize is the required key length in bytes (AES-256).
const KeySize = 32

// Key is a fixed-size AES-256 key.
type Key [KeySize]byte

// Encrypt encrypts plaintext with AES-256-GCM.
// Output format: base64(12-byte-nonce ‖ ciphertext ‖ 16-byte-GCM-tag).
// Each call produces a unique ciphertext because a fresh random nonce is
// generated per call.
func Encrypt(plaintext []byte, key Key) (string, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", fmt.Errorf("creating cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("creating GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generating nonce: %w", err)
	}

	// Seal appends ciphertext+tag after nonce.
	sealed := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt decrypts a value produced by Encrypt.
// Returns the original plaintext or an error if the key is wrong or the
// ciphertext has been tampered with (GCM authentication failure).
func Decrypt(encoded string, key Key) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decoding ciphertext: %w", err)
	}

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("creating cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize+gcm.Overhead() {
		return nil, fmt.Errorf("ciphertext too short (got %d bytes, minimum %d)", len(data), nonceSize+gcm.Overhead())
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypting: wrong key or corrupted ciphertext")
	}

	return plaintext, nil
}

// ParseKey parses a 64-character hex-encoded string into a Key.
// The hex encoding is canonical: openssl rand -hex 32.
func ParseKey(keyHex string) (Key, error) {
	var key Key
	b, err := hex.DecodeString(keyHex)
	if err != nil {
		return key, fmt.Errorf("invalid key: must be 64 hex characters: %w", err)
	}
	if len(b) != KeySize {
		return key, fmt.Errorf("invalid key length: want %d bytes (%d hex chars), got %d bytes",
			KeySize, KeySize*2, len(b))
	}
	copy(key[:], b)
	return key, nil
}

// GenerateKey generates a cryptographically random AES-256 key.
func GenerateKey() (Key, error) {
	var key Key
	if _, err := io.ReadFull(rand.Reader, key[:]); err != nil {
		return key, fmt.Errorf("generating key: %w", err)
	}
	return key, nil
}

// FormatKey returns the lowercase hex encoding of a key.
// This is the format expected by ParseKey.
func FormatKey(key Key) string {
	return hex.EncodeToString(key[:])
}
