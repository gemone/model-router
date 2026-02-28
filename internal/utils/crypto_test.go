package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptDecrypt(t *testing.T) {
	InitEncryptionKey("test-key-32-bytes-long-for-testing")

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
			assert.NotEmpty(t, encrypted)
			assert.NotEqual(t, tt.plaintext, encrypted)

			decrypted, err := Decrypt(encrypted)
			require.NoError(t, err)
			assert.Equal(t, tt.plaintext, decrypted)
		})
	}
}

func TestEncryptUnique(t *testing.T) {
	InitEncryptionKey("test-key")

	// Same plaintext should produce different ciphertexts due to random nonce
	plaintext := "test"
	encrypted1, _ := Encrypt(plaintext)
	encrypted2, _ := Encrypt(plaintext)

	assert.NotEqual(t, encrypted1, encrypted2)

	// But both should decrypt to the same plaintext
	decrypted1, _ := Decrypt(encrypted1)
	decrypted2, _ := Decrypt(encrypted2)

	assert.Equal(t, plaintext, decrypted1)
	assert.Equal(t, plaintext, decrypted2)
}

func TestDecryptInvalid(t *testing.T) {
	InitEncryptionKey("test-key")

	tests := []struct {
		name        string
		ciphertext  string
		expectedErr bool
	}{
		{"invalid base64", "!!!not-base64!!!", true},
		{"too short", "abcd", true},
		{"empty string", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decrypt(tt.ciphertext)
			if tt.expectedErr {
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
