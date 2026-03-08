package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

var encryptionKey []byte

// GenerateEncryptionKey 生成一个安全的随机加密密钥
func GenerateEncryptionKey() (string, error) {
	key := make([]byte, 32) // 256-bit key
	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("failed to generate encryption key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

// PromptForEncryptionKey 交互式提示用户选择加密密钥选项
func PromptForEncryptionKey() (string, error) {
	fmt.Println("\n=== Encryption Key Setup ===")
	fmt.Println("Choose an option:")
	fmt.Println("1. Generate a new random key (recommended)")
	fmt.Println("2. Enter an existing key")
	fmt.Print("Option (1 or 2): ")

	var choice string
	fmt.Scanln(&choice)

	choice = strings.TrimSpace(choice)

	if choice == "2" {
		fmt.Print("Enter your existing encryption key (base64 encoded): ")
		var key string
		fmt.Scanln(&key)
		return strings.TrimSpace(key), nil
	}

	// Generate new key
	key, err := GenerateEncryptionKey()
	if err != nil {
		return "", err
	}

	fmt.Println("\n=== Generated Encryption Key ===")
	fmt.Println("Save this key securely. You'll need it to decrypt your data:")
	fmt.Println(key)
	fmt.Println()

	return key, nil
}

// InitEncryptionKey 初始化加密密钥
func InitEncryptionKey(key string) error {
	if key == "" {
		return fmt.Errorf("encryption key cannot be empty")
	}

	decodedKey, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return fmt.Errorf("failed to decode encryption key: %w", err)
	}

	if len(decodedKey) != 32 {
		return fmt.Errorf("encryption key must be 32 bytes (256 bits) after base64 decoding, got %d bytes", len(decodedKey))
	}

	encryptionKey = decodedKey
	return nil
}

// Encrypt 加密数据
func Encrypt(plaintext string) (string, error) {
	if len(encryptionKey) == 0 {
		return "", fmt.Errorf("encryption not initialized")
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt 解密数据
func Decrypt(ciphertext string) (string, error) {
	if len(encryptionKey) == 0 {
		return "", fmt.Errorf("encryption not initialized")
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, cipherData := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, cipherData, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}

// ConstantTimeCompare 常量时间比较，防止时序攻击
func ConstantTimeCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// HashToken 使用 SHA256 哈希 token
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return base64.StdEncoding.EncodeToString(hash[:])
}

// GenerateRandomToken 生成随机 token
func GenerateRandomToken(length int) (string, error) {
	if length <= 0 {
		length = 32
	}

	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}

	return base64.URLEncoding.EncodeToString(bytes), nil
}

// ValidateEncryptionKey 验证加密密钥是否有效
func ValidateEncryptionKey(key string) error {
	if key == "" {
		return fmt.Errorf("encryption key is empty")
	}

	decodedKey, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return fmt.Errorf("invalid base64 encoding: %w", err)
	}

	if len(decodedKey) != 32 {
		return fmt.Errorf("encryption key must be 32 bytes (256 bits), got %d bytes", len(decodedKey))
	}

	return nil
}

// IsEncryptionEnabled 检查加密是否已启用
func IsEncryptionEnabled() bool {
	return len(encryptionKey) > 0
}

// RequireEncryption 要求加密必须启用，否则退出程序
func RequireEncryption() {
	if !IsEncryptionEnabled() {
		log.Printf("ERROR: Encryption is required but not enabled.")
		log.Printf("\nENCRYPTION_KEY environment variable is required for secure operation.")
		log.Printf("\nGenerate a secure key with one of these methods:")
		log.Printf("  openssl rand -base64 32")
		log.Printf("  OR")
		log.Printf("  ENCRYPTION_KEY=$(openssl rand -base64 32)")
		log.Printf("\nThen set it in your environment or .env file:")
		log.Printf("  ENCRYPTION_KEY=<your-generated-key>")
		os.Exit(1)
	}
}

// DeriveKeyFromPassword has been removed due to security concerns.
// For password-based key derivation, use golang.org/x/crypto/scrypt or Argon2.
// Example:
//
//	import "golang.org/x/crypto/scrypt"
//
//	func DeriveKeyFromPassword(password string, salt []byte) ([]byte, error) {
//		return scrypt.Key([]byte(password), salt, 32768, 8, 1, 32)
//	}
