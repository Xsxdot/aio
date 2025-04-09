// Package persistence 提供缓存系统的持久化功能
package persistence

import (
	"github.com/xsxdot/aio/pkg/common"
	"os"
	"path/filepath"
)

// EnsureDir 确保目录存在，如果不存在则创建
func EnsureDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0755)
}

// LogLevel 定义日志级别常量
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

// LogMsg 根据日志级别输出信息
// 已弃用：使用common.GetLogger()或对象内部的logger字段替代
// 每个管理器现在应该在初始化时获取日志记录器
func LogMsg(logLevel, minLevel int, format string, args ...interface{}) {
	if logLevel >= minLevel {
		logger := common.GetLogger()
		switch LogLevel(logLevel) {
		case DEBUG:
			logger.Debugf(format, args...)
		case INFO:
			logger.Infof(format, args...)
		case WARN:
			logger.Warnf(format, args...)
		case ERROR:
			logger.Errorf(format, args...)
		case FATAL:
			logger.Fatalf(format, args...)
		default:
			logger.Infof(format, args...)
		}
	}
}
