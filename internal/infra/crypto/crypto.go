// Package crypto 对称加密：敏感字段（如供应商 api_key）落库前加密、读取时解密。
//
// 设计：
//   - AES-256-GCM（认证加密，防篡改）
//   - 密钥来自 ENCRYPTION_KEY（infra/config），不进 git 不进日志
//   - 密钥长度不足 32 字节时 Hash 拉伸（开发期便利，生产应直接配 32 字节）
//   - 密文含随机 nonce，同明文每次加密结果不同（防比对猜测）
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"strings"
)

var errEmptyKey = errors.New("encryption key is empty")

// Encrypt 加密明文，返回 base64 密文（含 nonce 前缀）。
func Encrypt(plaintext, key string) (string, error) {
	if plaintext == "" {
		return "", nil // 空值不加密，保持空
	}
	if key == "" {
		return "", errEmptyKey
	}

	block, err := aes.NewCipher(keyHash(key))
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

// Decrypt 解密 Encrypt 产出的密文；空串原样返回。
func Decrypt(ciphertext, key string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	if key == "" {
		return "", errEmptyKey
	}

	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", errors.New("invalid ciphertext encoding")
	}

	block, err := aes.NewCipher(keyHash(key))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	size := gcm.NonceSize()
	if len(raw) < size {
		return "", errors.New("ciphertext too short")
	}
	plain, err := gcm.Open(nil, raw[:size], raw[size:], nil)
	if err != nil {
		return "", errors.New("decryption failed (key mismatch?)")
	}
	return string(plain), nil
}

// Mask 脱敏显示：只保留首尾各 4 位。空串返回空。
func Mask(key string) string {
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}

// keyHash 把任意长度密钥拉成 32 字节（AES-256）。
func keyHash(key string) []byte {
	h := sha256.Sum256([]byte(key))
	return h[:]
}
