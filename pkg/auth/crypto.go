package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

// GenerateRSAKey 生成RSA密钥对
func GenerateRSAKey(bits int) (*rsa.PrivateKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, err
	}
	return privateKey, nil
}

// EncodeRSAPrivateKeyToPEM 将RSA私钥编码为PEM格式
func EncodeRSAPrivateKeyToPEM(privateKey *rsa.PrivateKey) ([]byte, error) {
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	}
	return pem.EncodeToMemory(block), nil
}

// EncodeRSAPublicKeyToPEM 将RSA公钥编码为PEM格式
func EncodeRSAPublicKeyToPEM(publicKey *rsa.PublicKey) ([]byte, error) {
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, err
	}
	block := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	}
	return pem.EncodeToMemory(block), nil
}

// DecodeRSAPrivateKeyFromPEM 从PEM格式解码RSA私钥
func DecodeRSAPrivateKeyFromPEM(privateKeyPEM []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(privateKeyPEM)
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		return nil, errors.New("无效的私钥PEM格式")
	}
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return privateKey, nil
}

// DecodeRSAPublicKeyFromPEM 从PEM格式解码RSA公钥
func DecodeRSAPublicKeyFromPEM(publicKeyPEM []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(publicKeyPEM)
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, errors.New("无效的公钥PEM格式")
	}
	publicKeyInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	publicKey, ok := publicKeyInterface.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("不是有效的RSA公钥")
	}
	return publicKey, nil
}

// LoadRSAPrivateKeyFromFile 从文件加载RSA私钥
func LoadRSAPrivateKeyFromFile(path string) (*rsa.PrivateKey, error) {
	privateKeyPEM, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取私钥文件失败: %w", err)
	}
	return DecodeRSAPrivateKeyFromPEM(privateKeyPEM)
}

// LoadRSAPublicKeyFromFile 从文件加载RSA公钥
func LoadRSAPublicKeyFromFile(path string) (*rsa.PublicKey, error) {
	publicKeyPEM, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取公钥文件失败: %w", err)
	}
	return DecodeRSAPublicKeyFromPEM(publicKeyPEM)
}

// SaveRSAPrivateKeyToFile 将RSA私钥保存到文件
func SaveRSAPrivateKeyToFile(privateKey *rsa.PrivateKey, path string) error {
	privateKeyPEM, err := EncodeRSAPrivateKeyToPEM(privateKey)
	if err != nil {
		return fmt.Errorf("编码私钥失败: %w", err)
	}
	return ioutil.WriteFile(path, privateKeyPEM, 0600)
}

// SaveRSAPublicKeyToFile 将RSA公钥保存到文件
func SaveRSAPublicKeyToFile(publicKey *rsa.PublicKey, path string) error {
	publicKeyPEM, err := EncodeRSAPublicKeyToPEM(publicKey)
	if err != nil {
		return fmt.Errorf("编码公钥失败: %w", err)
	}
	return ioutil.WriteFile(path, publicKeyPEM, 0644)
}

// GenerateAndSaveRSAKeyPair 生成并保存RSA密钥对
func GenerateAndSaveRSAKeyPair(bits int, privateKeyPath, publicKeyPath string) (*RSAKeyPair, error) {
	// 生成RSA密钥对
	keyPair, err := GenerateRSAKeyPair(bits)
	if err != nil {
		return nil, err
	}

	if err = os.MkdirAll(filepath.Dir(privateKeyPath), 0700); err != nil {
		return nil, fmt.Errorf("创建证书密钥目录失败: %w", err)
	}

	// 保存私钥
	if err := SaveRSAPrivateKeyToFile(keyPair.PrivateKey, privateKeyPath); err != nil {
		return nil, fmt.Errorf("保存私钥失败: %w", err)
	}

	// 保存公钥
	if err := SaveRSAPublicKeyToFile(keyPair.PublicKey, publicKeyPath); err != nil {
		return nil, fmt.Errorf("保存公钥失败: %w", err)
	}

	return keyPair, nil
}

// LoadRSAKeyPairFromFiles 从文件加载RSA密钥对
func LoadRSAKeyPairFromFiles(privateKeyPath, publicKeyPath string) (*RSAKeyPair, error) {
	// 加载私钥
	privateKey, err := LoadRSAPrivateKeyFromFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("加载私钥失败: %w", err)
	}

	// 加载公钥
	publicKey, err := LoadRSAPublicKeyFromFile(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("加载公钥失败: %w", err)
	}

	// 编码私钥
	privateKeyPEM, err := EncodeRSAPrivateKeyToPEM(privateKey)
	if err != nil {
		return nil, fmt.Errorf("编码私钥失败: %w", err)
	}

	// 编码公钥
	publicKeyPEM, err := EncodeRSAPublicKeyToPEM(publicKey)
	if err != nil {
		return nil, fmt.Errorf("编码公钥失败: %w", err)
	}

	// 创建RSA密钥对
	keyPair := &RSAKeyPair{
		PrivateKey:    privateKey,
		PrivateKeyPEM: string(privateKeyPEM),
		PublicKey:     publicKey,
		PublicKeyPEM:  string(publicKeyPEM),
	}

	return keyPair, nil
}

// CheckFileExistence 检查文件是否存在
func CheckFileExistence(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}
