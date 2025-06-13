package grpc

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// AuthProvider 鉴权提供者接口
type AuthProvider interface {
	// VerifyToken 验证令牌并返回认证信息
	VerifyToken(token string) (*AuthInfo, error)
	// VerifyPermission 验证权限
	VerifyPermission(token string, resource, action string) (*PermissionResult, error)
}

// AuthInfo 认证信息
type AuthInfo struct {
	SubjectID   string                 `json:"subject_id"`
	SubjectType string                 `json:"subject_type"`
	Name        string                 `json:"name"`
	Roles       []string               `json:"roles"`
	Permissions []Permission           `json:"permissions"`
	Extra       map[string]interface{} `json:"extra,omitempty"`
}

// Permission 权限定义
type Permission struct {
	Resource string `json:"resource"`
	Action   string `json:"action"`
}

// PermissionResult 权限验证结果
type PermissionResult struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

// AuthConfig 鉴权配置
type AuthConfig struct {
	// SkipMethods 跳过鉴权的方法列表
	SkipMethods []string
	// RequireAuth 是否要求认证（默认true）
	RequireAuth bool
	// AuthProvider 鉴权提供者
	AuthProvider AuthProvider
}

// unaryLoggingInterceptor 统一的日志记录中间件
func unaryLoggingInterceptor(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()

		// 记录请求开始
		logger.Debug("gRPC 请求开始",
			zap.String("method", info.FullMethod),
			zap.Time("start_time", start))

		// 调用处理函数
		resp, err := handler(ctx, req)

		// 计算耗时
		duration := time.Since(start)

		// 获取状态码
		code := codes.OK
		if err != nil {
			if st, ok := status.FromError(err); ok {
				code = st.Code()
			} else {
				code = codes.Internal
			}
		}

		// 记录请求完成
		if err != nil {
			logger.Error("gRPC 请求完成",
				zap.String("method", info.FullMethod),
				zap.Duration("duration", duration),
				zap.String("status", code.String()),
				zap.Error(err))
		} else {
			logger.Info("gRPC 请求完成",
				zap.String("method", info.FullMethod),
				zap.Duration("duration", duration),
				zap.String("status", code.String()))
		}

		return resp, err
	}
}

// AuthInterceptor 鉴权中间件
func AuthInterceptor(config *AuthConfig, logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// 检查是否跳过鉴权
		if shouldSkipAuth(info.FullMethod, config.SkipMethods) {
			logger.Debug("跳过鉴权", zap.String("method", info.FullMethod))
			return handler(ctx, req)
		}

		// 如果没有配置鉴权提供者且要求认证，返回错误
		if config.AuthProvider == nil && config.RequireAuth {
			logger.Error("未配置鉴权提供者", zap.String("method", info.FullMethod))
			return nil, status.Error(codes.Internal, "鉴权服务未配置")
		}

		// 从 metadata 中提取令牌
		token, err := extractToken(ctx)
		if err != nil {
			logger.Warn("提取令牌失败",
				zap.String("method", info.FullMethod),
				zap.Error(err))
			if config.RequireAuth {
				return nil, status.Error(codes.Unauthenticated, "缺少认证令牌")
			}
			// 如果不要求认证，继续执行
			return handler(ctx, req)
		}

		// 验证令牌
		authInfo, err := config.AuthProvider.VerifyToken(token)
		if err != nil {
			logger.Warn("令牌验证失败",
				zap.String("method", info.FullMethod),
				zap.Error(err))
			if config.RequireAuth {
				return nil, status.Error(codes.Unauthenticated, "无效的认证令牌")
			}
			// 如果不要求认证，继续执行
			return handler(ctx, req)
		}

		// 将认证信息添加到上下文
		ctx = context.WithValue(ctx, "authInfo", authInfo)
		ctx = context.WithValue(ctx, "token", token)

		logger.Debug("鉴权成功",
			zap.String("method", info.FullMethod),
			zap.String("subject_id", authInfo.SubjectID),
			zap.String("subject_type", authInfo.SubjectType))

		// 继续执行
		return handler(ctx, req)
	}
}

// PermissionInterceptor 权限验证中间件
func PermissionInterceptor(config *AuthConfig, logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// 检查是否跳过权限验证
		if shouldSkipAuth(info.FullMethod, config.SkipMethods) {
			return handler(ctx, req)
		}

		// 获取令牌和认证信息
		token, ok := ctx.Value("token").(string)
		if !ok && config.RequireAuth {
			return nil, status.Error(codes.Unauthenticated, "缺少认证信息")
		}

		authInfo, ok := ctx.Value("authInfo").(*AuthInfo)
		if !ok && config.RequireAuth {
			return nil, status.Error(codes.Unauthenticated, "缺少认证信息")
		}

		// 如果有令牌和认证信息，进行权限验证
		if token != "" && authInfo != nil && config.AuthProvider != nil {
			// 解析方法名为资源和操作
			resource, action := parseMethodPermission(info.FullMethod)

			// 验证权限
			result, err := config.AuthProvider.VerifyPermission(token, resource, action)
			if err != nil {
				logger.Error("权限验证失败",
					zap.String("method", info.FullMethod),
					zap.String("resource", resource),
					zap.String("action", action),
					zap.Error(err))
				return nil, status.Error(codes.Internal, "权限验证失败")
			}

			if !result.Allowed {
				logger.Warn("权限不足",
					zap.String("method", info.FullMethod),
					zap.String("resource", resource),
					zap.String("action", action),
					zap.String("subject_id", authInfo.SubjectID),
					zap.String("reason", result.Reason))
				return nil, status.Error(codes.PermissionDenied, "权限不足")
			}

			logger.Debug("权限验证通过",
				zap.String("method", info.FullMethod),
				zap.String("resource", resource),
				zap.String("action", action),
				zap.String("subject_id", authInfo.SubjectID))
		}

		return handler(ctx, req)
	}
}

// shouldSkipAuth 检查是否应该跳过鉴权
func shouldSkipAuth(method string, skipMethods []string) bool {
	for _, skipMethod := range skipMethods {
		if method == skipMethod || strings.HasSuffix(method, skipMethod) {
			return true
		}
	}
	return false
}

// extractToken 从 gRPC metadata 中提取令牌
func extractToken(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "缺少 metadata")
	}

	// 尝试从 authorization header 中提取
	values := md.Get("authorization")
	if len(values) == 0 {
		// 尝试从 token header 中提取
		values = md.Get("token")
		if len(values) == 0 {
			return "", status.Error(codes.Unauthenticated, "缺少认证令牌")
		}
	}

	token := values[0]

	// 移除 Bearer 前缀（如果有）
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = token[7:]
	}

	if token == "" {
		return "", status.Error(codes.Unauthenticated, "空的认证令牌")
	}

	return token, nil
}

// parseMethodPermission 解析方法名为资源和操作
func parseMethodPermission(fullMethod string) (resource, action string) {
	// 格式: /package.service/Method
	// 例如: /auth.v1.AuthService/ClientAuth

	parts := strings.Split(fullMethod, "/")
	if len(parts) != 3 {
		return fullMethod, "invoke"
	}

	service := parts[1] // package.service
	method := parts[2]  // Method

	// 将服务名作为资源，方法名作为操作
	return service, strings.ToLower(method)
}

// RecoveryInterceptor 恢复中间件，防止 panic 导致服务崩溃
func RecoveryInterceptor(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("gRPC 处理发生 panic",
					zap.String("method", info.FullMethod),
					zap.Any("panic", r))
				err = status.Error(codes.Internal, "内部服务器错误")
			}
		}()

		return handler(ctx, req)
	}
}

// ValidationInterceptor 参数验证中间件
func ValidationInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// 如果请求实现了 Validate 方法，则调用验证
		if validator, ok := req.(interface{ Validate() error }); ok {
			if err := validator.Validate(); err != nil {
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
		}

		return handler(ctx, req)
	}
}

// ChainUnaryInterceptors 链式组合多个拦截器
func ChainUnaryInterceptors(interceptors ...grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		chainer := func(currentInter grpc.UnaryServerInterceptor, currentHandler grpc.UnaryHandler) grpc.UnaryHandler {
			return func(currentCtx context.Context, currentReq interface{}) (interface{}, error) {
				return currentInter(currentCtx, currentReq, info, currentHandler)
			}
		}

		chainedHandler := handler
		for i := len(interceptors) - 1; i >= 0; i-- {
			chainedHandler = chainer(interceptors[i], chainedHandler)
		}

		return chainedHandler(ctx, req)
	}
}
