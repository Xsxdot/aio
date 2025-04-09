package common

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LogLevel 日志级别类型
type LogLevel string

// 日志级别常量
const (
	DebugLevel LogLevel = "debug"
	InfoLevel  LogLevel = "info"
	WarnLevel  LogLevel = "warn"
	ErrorLevel LogLevel = "error"
	FatalLevel LogLevel = "fatal"
)

// LogConfig 日志配置
type LogConfig struct {
	Level      LogLevel `yaml:"level"`       // 日志级别
	Filename   string   `yaml:"file"`        // 日志文件路径
	MaxSize    int      `yaml:"max_size"`    // 单个日志文件最大大小（MB）
	MaxBackups int      `yaml:"max_backups"` // 最大保留历史日志文件数
	MaxAge     int      `yaml:"max_age"`     // 日志文件保留天数
	Compress   bool     `yaml:"compress"`    // 是否压缩历史日志
	Console    bool     `yaml:"console"`     // 是否同时输出到控制台
}

// Logger 包装了zap.Logger提供统一的日志接口
type Logger struct {
	zap  *zap.Logger
	atom zap.AtomicLevel
}

var (
	defaultLogger *Logger
)

// 初始化默认的日志配置
func init() {
	cfg := LogConfig{
		Level:      InfoLevel,
		Filename:   "",
		MaxSize:    100,
		MaxBackups: 3,
		MaxAge:     7,
		Compress:   false,
		Console:    true,
	}
	logger, err := NewLogger(cfg)
	if err != nil {
		panic(fmt.Sprintf("初始化默认日志器失败: %v", err))
	}
	defaultLogger = logger
}

// GetLogger 获取默认的日志器
func GetLogger() *Logger {
	return defaultLogger
}

// GetZapLogger 获取默认的日志器
func (l *Logger) GetZapLogger(name string) *zap.Logger {
	return l.ZapLogger().Named(name)
}

// SetLogger 设置默认的日志器
func SetLogger(logger *Logger) {
	defaultLogger = logger
}

// NewLogger 创建新的日志器
func NewLogger(cfg LogConfig) (*Logger, error) {
	var writers []io.Writer
	var rotateLogger *lumberjack.Logger

	level := getZapLevel(cfg.Level)
	atom := zap.NewAtomicLevelAt(level)

	encoderCfg := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// 如果指定了日志文件，则配置文件输出
	if cfg.Filename != "" {
		// 确保日志目录存在
		logDir := filepath.Dir(cfg.Filename)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return nil, fmt.Errorf("创建日志目录失败: %w", err)
		}

		// 配置日志文件轮转
		rotateLogger = &lumberjack.Logger{
			Filename:   cfg.Filename,
			MaxSize:    cfg.MaxSize,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAge,
			Compress:   cfg.Compress,
		}
		writers = append(writers, rotateLogger)
	}

	// 如果需要同时输出到控制台
	if cfg.Console {
		writers = append(writers, os.Stdout)
	}

	// 如果没有配置任何输出，默认输出到控制台
	if len(writers) == 0 {
		writers = append(writers, os.Stdout)
	}

	// 创建多路复用的writer
	multiWriter := io.MultiWriter(writers...)

	// 创建核心
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderCfg),
		zapcore.AddSync(multiWriter),
		atom,
	)

	// 创建logger
	zapLogger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))

	return &Logger{
		zap:  zapLogger,
		atom: atom,
	}, nil
}

// getZapLevel 将自定义日志级别转换为zap的日志级别
func getZapLevel(level LogLevel) zapcore.Level {
	switch level {
	case DebugLevel:
		return zapcore.DebugLevel
	case InfoLevel:
		return zapcore.InfoLevel
	case WarnLevel:
		return zapcore.WarnLevel
	case ErrorLevel:
		return zapcore.ErrorLevel
	case FatalLevel:
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}

// SetLevel 动态设置日志级别
func (l *Logger) SetLevel(level LogLevel) {
	l.atom.SetLevel(getZapLevel(level))
}

// Debug 输出调试级别日志
func (l *Logger) Debug(msg string, fields ...zapcore.Field) {
	l.zap.Debug(msg, fields...)
}

// Info 输出信息级别日志
func (l *Logger) Info(msg string, fields ...zapcore.Field) {
	l.zap.Info(msg, fields...)
}

// Warn 输出警告级别日志
func (l *Logger) Warn(msg string, fields ...zapcore.Field) {
	l.zap.Warn(msg, fields...)
}

// Error 输出错误级别日志
func (l *Logger) Error(msg string, fields ...zapcore.Field) {
	l.zap.Error(msg, fields...)
}

// Fatal 输出致命级别日志
func (l *Logger) Fatal(msg string, fields ...zapcore.Field) {
	l.zap.Fatal(msg, fields...)
}

// Debugf 使用格式化字符串输出调试级别日志
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.zap.Debug(fmt.Sprintf(format, args...))
}

// Infof 使用格式化字符串输出信息级别日志
func (l *Logger) Infof(format string, args ...interface{}) {
	l.zap.Info(fmt.Sprintf(format, args...))
}

// Warnf 使用格式化字符串输出警告级别日志
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.zap.Warn(fmt.Sprintf(format, args...))
}

// Errorf 使用格式化字符串输出错误级别日志
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.zap.Error(fmt.Sprintf(format, args...))
}

// Fatalf 使用格式化字符串输出致命级别日志
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.zap.Fatal(fmt.Sprintf(format, args...))
}

// With 创建带有指定字段的日志记录器
func (l *Logger) With(fields ...zapcore.Field) *Logger {
	return &Logger{
		zap:  l.zap.With(fields...),
		atom: l.atom,
	}
}

// WithField 创建带有单个字段的日志记录器
func (l *Logger) WithField(key string, value interface{}) *Logger {
	return l.With(zap.Any(key, value))
}

// WithFields 创建带有多个字段的日志记录器
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	zapFields := make([]zapcore.Field, 0, len(fields))
	for k, v := range fields {
		zapFields = append(zapFields, zap.Any(k, v))
	}
	return l.With(zapFields...)
}

// Sync 同步日志缓冲区
func (l *Logger) Sync() error {
	return l.zap.Sync()
}

// ZapLogger 获取底层的zap.Logger实例
func (l *Logger) ZapLogger() *zap.Logger {
	return l.zap
}

// Debug 输出调试级别日志（包级别函数）
func Debug(msg string, fields ...zapcore.Field) {
	defaultLogger.Debug(msg, fields...)
}

// Info 输出信息级别日志（包级别函数）
func Info(msg string, fields ...zapcore.Field) {
	defaultLogger.Info(msg, fields...)
}

// Warn 输出警告级别日志（包级别函数）
func Warn(msg string, fields ...zapcore.Field) {
	defaultLogger.Warn(msg, fields...)
}

// Error 输出错误级别日志（包级别函数）
func Error(msg string, fields ...zapcore.Field) {
	defaultLogger.Error(msg, fields...)
}

// Fatal 输出致命级别日志（包级别函数）
func Fatal(msg string, fields ...zapcore.Field) {
	defaultLogger.Fatal(msg, fields...)
}

// Debugf 使用格式化字符串输出调试级别日志（包级别函数）
func Debugf(format string, args ...interface{}) {
	defaultLogger.Debugf(format, args...)
}

// Infof 使用格式化字符串输出信息级别日志（包级别函数）
func Infof(format string, args ...interface{}) {
	defaultLogger.Infof(format, args...)
}

// Warnf 使用格式化字符串输出警告级别日志（包级别函数）
func Warnf(format string, args ...interface{}) {
	defaultLogger.Warnf(format, args...)
}

// Errorf 使用格式化字符串输出错误级别日志（包级别函数）
func Errorf(format string, args ...interface{}) {
	defaultLogger.Errorf(format, args...)
}

// Fatalf 使用格式化字符串输出致命级别日志（包级别函数）
func Fatalf(format string, args ...interface{}) {
	defaultLogger.Fatalf(format, args...)
}

// Field 创建日志字段，封装zap.Field类型
type Field = zapcore.Field

// 导出常用的字段构造函数
var (
	Any        = zap.Any
	Bool       = zap.Bool
	Duration   = zap.Duration
	Float64    = zap.Float64
	Int        = zap.Int
	Int64      = zap.Int64
	String     = zap.String
	Uint       = zap.Uint
	Uint64     = zap.Uint64
	Time       = zap.Time
	ErrorField = zap.Error // 重命名为ErrorField避免冲突
	StackTrace = zap.StackSkip
	Reflect    = zap.Reflect
)
