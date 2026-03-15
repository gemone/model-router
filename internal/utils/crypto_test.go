package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptDecrypt(t *testing.T) {
	// Use base64-encoded 32-byte key
	err := InitEncryptionKey("MTIzNDU2Nzg5MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTI=")
	require.NoError(t, err)

	tests := []struct {
		name      string
		plaintext string
	}{
		{"simple text", "Hello, World!"},
		{"empty string", ""},
		{"long text", "This is a very long text that should be encrypted and decrypted correctly without any issues."},
		{"special chars", "!@#$%^&*()_+-=[]{}|;':\",./<>?"},
		{"unicode", "你好世界 🌍 Привет мир"},
		{"api key", "sk-abcdefghijklmnopqrstuvwxyz1234567890"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encrypted, err := Encrypt(tt.plaintext)
			require.NoError(t, err)
			// Encrypted should be different from plaintext (unless empty)
			if tt.plaintext != "" {
				assert.NotEqual(t, tt.plaintext, encrypted)
			}

			decrypted, err := Decrypt(encrypted)
			require.NoError(t, err)
			assert.Equal(t, tt.plaintext, decrypted)
		})
	}
}

func TestEncryptUnique(t *testing.T) {
	// Use base64-encoded 32-byte key
	err := InitEncryptionKey("MTIzNDU2Nzg5MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTI=")
	require.NoError(t, err)

	// Each encryption should produce unique ciphertext (due to random nonce)
	plaintext := "test"
	encrypted1, err := Encrypt(plaintext)
	require.NoError(t, err)
	encrypted2, err := Encrypt(plaintext)
	require.NoError(t, err)

	// Two encryptions of same plaintext should produce different ciphertexts
	assert.NotEqual(t, encrypted1, encrypted2)

	// But both should decrypt to the same plaintext
	decrypted1, err := Decrypt(encrypted1)
	require.NoError(t, err)
	decrypted2, err := Decrypt(encrypted2)
	require.NoError(t, err)

	assert.Equal(t, plaintext, decrypted1)
	assert.Equal(t, plaintext, decrypted2)
}

func TestDecryptInvalid(t *testing.T) {
	// Use base64-encoded 32-byte key
	err := InitEncryptionKey("MTIzNDU2Nzg5MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTI=")
	require.NoError(t, err)

	tests := []struct {
		name       string
		ciphertext string
		expectErr  bool
	}{
		{"invalid base64", "not-valid-base64!!!", true},
		{"empty string", "", true},
		{"too short", "abc", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decrypt(tt.ciphertext)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConstantTimeCompare(t *testing.T) {
	tests := []struct {
		a        string
		b        string
		expected bool
	}{
		{"same", "same", true},
		{"different", "other", false},
		{"", "", true},
		{"case", "CASE", false},
		{"longer string", "longer string", true},
		{"longer string", "shorter", false},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			result := ConstantTimeCompare(tt.a, tt.b)
			assert.Equal(t, tt.expected, result)
		})
	}
}
