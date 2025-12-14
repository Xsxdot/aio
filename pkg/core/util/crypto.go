package util

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	errorc "xiaozhizhang/pkg/core/err"
)

const EncryptedPrefix = "ENC:"

// EncryptAES 使用 AES-256-GCM 加密明文
// plaintext: 待加密的明文
// salt: 加密盐值，用于生成密钥
// 返回: 加密后的字符串（带 ENC: 前缀）或错误
func EncryptAES(plaintext, salt string) (string, error) {
	// 1. 检查是否已加密，避免重复加密
	if strings.HasPrefix(plaintext, EncryptedPrefix) {
		return plaintext, nil
	}

	// 2. 使用 salt 通过 SHA-256 生成 32 字节密钥
	key := sha256.Sum256([]byte(salt))

	// 3. 创建 AES cipher
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", errorc.New("创建AES cipher失败", err)
	}

	// 4. 创建 GCM 模式
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", errorc.New("创建GCM模式失败", err)
	}

	// 5. 生成随机 nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", errorc.New("生成nonce失败", err)
	}

	// 6. 加密数据（nonce + ciphertext + tag）
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// 7. Base64 编码并添加前缀
	encoded := base64.StdEncoding.EncodeToString(ciphertext)
	return EncryptedPrefix + encoded, nil
}

// DecryptAES 使用 AES-256-GCM 解密密文
// ciphertext: 待解密的密文（带 ENC: 前缀）
// salt: 解密盐值，必须与加密时使用的相同
// 返回: 解密后的明文或错误
func DecryptAES(ciphertext, salt string) (string, error) {
	// 1. 检查前缀
	if !strings.HasPrefix(ciphertext, EncryptedPrefix) {
		// 如果没有前缀，认为是明文，直接返回
		return ciphertext, nil
	}

	// 2. 去除前缀并 Base64 解码
	encoded := strings.TrimPrefix(ciphertext, EncryptedPrefix)
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", errorc.New("Base64解码失败", err)
	}

	// 3. 使用 salt 生成密钥
	key := sha256.Sum256([]byte(salt))

	// 4. 创建 AES cipher
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", errorc.New("创建AES cipher失败", err)
	}

	// 5. 创建 GCM 模式
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", errorc.New("创建GCM模式失败", err)
	}

	// 6. 提取 nonce
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errorc.New("密文长度不足", nil)
	}

	nonce, ciphertextData := data[:nonceSize], data[nonceSize:]

	// 7. 解密数据
	plaintext, err := gcm.Open(nil, nonce, ciphertextData, nil)
	if err != nil {
		return "", errorc.New("解密失败", err)
	}

	return string(plaintext), nil
}

// IsEncrypted 检查字符串是否已加密
func IsEncrypted(text string) bool {
	return strings.HasPrefix(text, EncryptedPrefix)
}
