package certmanager

import (
	"fmt"
	"os"
	"path/filepath"
)

// createDirIfNotExist 确保目录存在，如果不存在则创建
func createDirIfNotExist(dir string) error {
	// 检查目录是否存在
	_, err := os.Stat(dir)
	if err == nil {
		return nil // 目录已存在
	}

	if !os.IsNotExist(err) {
		return err // 发生其他错误
	}

	// 创建目录（及所有必需的父目录）
	return os.MkdirAll(dir, 0755)
}

// getLatestCertFiles 获取域名的最新证书文件
func getLatestCertFiles(domain string, certDir string) (certPath, keyPath string, err error) {
	domainDir := filepath.Join(certDir, domain)

	// 检查目录是否存在
	if _, err := os.Stat(domainDir); os.IsNotExist(err) {
		return "", "", fmt.Errorf("域名目录不存在")
	}

	// 读取目录
	files, err := os.ReadDir(domainDir)
	if err != nil {
		return "", "", fmt.Errorf("读取域名目录失败: %v", err)
	}

	// 查找最新的证书和私钥文件
	var latestCert, latestKey string
	var latestTime int64

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filename := file.Name()
		path := filepath.Join(domainDir, filename)

		info, err := os.Stat(path)
		if err != nil {
			continue
		}

		// 检查文件修改时间，选择最新的
		if info.ModTime().Unix() > latestTime {
			if filepath.Ext(filename) == ".crt" {
				latestCert = path
				latestTime = info.ModTime().Unix()
			} else if filepath.Ext(filename) == ".key" {
				latestKey = path
			}
		}
	}

	if latestCert == "" || latestKey == "" {
		return "", "", fmt.Errorf("未找到证书文件")
	}

	return latestCert, latestKey, nil
}
