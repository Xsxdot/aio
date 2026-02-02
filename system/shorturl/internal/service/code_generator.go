package service

import (
	"crypto/rand"
	"math/big"
)

const base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// GenerateShortCode 生成指定长度的随机短码（base62编码）
func GenerateShortCode(length int) (string, error) {
	if length <= 0 {
		length = 6 // 默认长度
	}

	result := make([]byte, length)
	max := big.NewInt(int64(len(base62Chars)))

	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		result[i] = base62Chars[n.Int64()]
	}

	return string(result), nil
}

