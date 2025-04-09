package utils

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteContentToFile 将内容写入指定文件并返回文件的绝对路径
// filename: 文件名称
// content: 要写入的内容（字节数组）
// perm: 文件权限，如果为0则默认使用0644
// 返回值: 文件的绝对路径和可能的错误
func WriteContentToFile(filename string, content []byte, perm os.FileMode) (string, error) {
	// 如果权限为0，设置为默认权限0644
	if perm == 0 {
		perm = 0644
	}

	// 确保目录存在
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("创建目录失败: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(filename, content, perm); err != nil {
		return "", fmt.Errorf("写入文件失败: %w", err)
	}

	// 获取文件的绝对路径
	absPath, err := filepath.Abs(filename)
	if err != nil {
		return "", fmt.Errorf("获取绝对路径失败: %w", err)
	}

	return absPath, nil
}

// WriteStringToFile 将字符串内容写入指定文件并返回文件的绝对路径
// filename: 文件名称
// content: 要写入的字符串内容
// perm: 文件权限，如果为0则默认使用0644
// 返回值: 文件的绝对路径和可能的错误
func WriteStringToFile(filename string, content string, perm os.FileMode) (string, error) {
	return WriteContentToFile(filename, []byte(content), perm)
}
