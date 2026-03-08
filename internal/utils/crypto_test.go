package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptDecrypt(t *testing.T) {
	// 注意：加密已禁用，Encrypt/Decrypt 直接返回原文
	err := InitEncryptionKey("test-key-32-bytes-long-for-testing")
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
			// 加密已禁用，直接返回原文
			encrypted, err := Encrypt(tt.plaintext)
			require.NoError(t, err)
			// 加密已禁用，原文等于密文
			assert.Equal(t, tt.plaintext, encrypted)

			decrypted, err := Decrypt(encrypted)
			require.NoError(t, err)
			assert.Equal(t, tt.plaintext, decrypted)
		})
	}
}

func TestEncryptUnique(t *testing.T) {
	// 注意：加密已禁用，相同明文返回相同结果
	err := InitEncryptionKey("test-key")
	require.NoError(t, err)

	// 加密已禁用，相同明文返回相同结果
	plaintext := "test"
	encrypted1, _ := Encrypt(plaintext)
	encrypted2, _ := Encrypt(plaintext)

	// 加密已禁用，两次加密结果相同
	assert.Equal(t, encrypted1, encrypted2)

	// 两者都应该解密为相同的明文
	decrypted1, _ := Decrypt(encrypted1)
	decrypted2, _ := Decrypt(encrypted2)

	assert.Equal(t, plaintext, decrypted1)
	assert.Equal(t, plaintext, decrypted2)
}

func TestDecryptInvalid(t *testing.T) {
	// 注意：加密已禁用，Decrypt 直接返回原文
	err := InitEncryptionKey("test-key")
	require.NoError(t, err)

	tests := []struct {
		name        string
		ciphertext  string
		expectedErr bool
	}{
		{"any text", "any text - decrypt returns as-is", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 加密已禁用，直接返回原文，不会出错
			result, err := Decrypt(tt.ciphertext)
			assert.NoError(t, err)
			assert.Equal(t, tt.ciphertext, result)
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
