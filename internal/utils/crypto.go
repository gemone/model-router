package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"io"
)

var encryptionKey []byte

// InitEncryptionKey 初始化加密密钥
func InitEncryptionKey(key string) {
	if key == "" {
		key = "model-router-default-encryption-key-32bytes"
	}
	// 确保密钥长度为32字节（AES-256）
	encryptionKey = []byte(key)
	if len(encryptionKey) < 32 {
		// 填充到32字节
		newKey := make([]byte, 32)
		copy(newKey, encryptionKey)
		encryptionKey = newKey
	} else if len(encryptionKey) > 32 {
		encryptionKey = encryptionKey[:32]
	}
}

// Encrypt 加密数据
func Encrypt(plaintext string) (string, error) {
	if encryptionKey == nil {
		InitEncryptionKey("")
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt 解密数据
func Decrypt(ciphertext string) (string, error) {
	if encryptionKey == nil {
		InitEncryptionKey("")
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// ConstantTimeCompare 常量时间比较，防止时序攻击
func ConstantTimeCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
