package utils

import (
	"github.com/gofiber/fiber/v2"
)

// 定义常用的状态码
const (
	// 成功状态码，与前端约定为20000
	StatusSuccess = 20000
	// 参数错误状态码
	StatusBadRequest = 40000
	// 未授权状态码
	StatusUnauthorized = 40100
	// 权限不足状态码
	StatusForbidden = 40300
	// 资源不存在状态码
	StatusNotFound = 40400
	// 服务器内部错误状态码
	StatusInternalError = 50000
	// 服务不可用状态码
	StatusServiceUnavailable = 50300
	// Token过期状态码，对应前端拦截器中的50014
	StatusTokenExpired = 50014
	// 非法Token状态码，对应前端拦截器中的50008
	StatusIllegalToken = 50008
	// 其他客户端登录状态码，对应前端拦截器中的50012
	StatusOtherLogin = 50012
)

// Response 统一返回结构体
type Response struct {
	// 状态码，与前端约定为20000表示成功
	Code int `json:"code"`
	// 消息内容
	Msg string `json:"msg"`
	// 数据内容
	Data interface{} `json:"data,omitempty"`
}

// NewResponse 创建新的响应
func NewResponse(code int, msg string, data interface{}) *Response {
	return &Response{
		Code: code,
		Msg:  msg,
		Data: data,
	}
}

// Success 返回成功响应
func Success(data interface{}) *Response {
	return NewResponse(StatusSuccess, "success", data)
}

// Fail 返回失败响应
func Fail(code int, msg string) *Response {
	return NewResponse(code, msg, nil)
}

// ResponseMiddleware 创建统一响应中间件
func ResponseMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 继续处理请求
		err := c.Next()

		// 如果已经发送了响应，直接返回
		if c.Response().StatusCode() != fiber.StatusOK {
			return err
		}

		// 获取当前上下文中的响应数据
		// 如果未设置，则返回默认成功响应
		resp, ok := c.Locals("response").(*Response)
		if !ok {
			resp = Success(nil)
		}

		return c.JSON(resp)
	}
}

// WithResponse 封装响应的辅助函数
func WithResponse(c *fiber.Ctx, resp *Response) error {
	return c.Status(fiber.StatusOK).JSON(resp)
}

// SuccessResponse 返回成功响应的辅助函数
func SuccessResponse(c *fiber.Ctx, data interface{}) error {
	return WithResponse(c, Success(data))
}

// FailResponse 返回失败响应的辅助函数
func FailResponse(c *fiber.Ctx, code int, msg string) error {
	return WithResponse(c, Fail(code, msg))
}

// ErrorResponse 由错误生成的失败响应
func ErrorResponse(c *fiber.Ctx, err error) error {
	code := StatusInternalError
	msg := "服务器内部错误"

	// 处理Fiber的错误
	if e, ok := err.(*fiber.Error); ok {
		switch e.Code {
		case fiber.StatusBadRequest:
			code = StatusBadRequest
			msg = "参数错误"
		case fiber.StatusUnauthorized:
			code = StatusUnauthorized
			msg = "未授权"
		case fiber.StatusForbidden:
			code = StatusForbidden
			msg = "禁止访问"
		case fiber.StatusNotFound:
			code = StatusNotFound
			msg = "资源不存在"
		}

		if e.Message != "" {
			msg = e.Message
		}
	} else if err != nil {
		msg = err.Error()
	}

	return WithResponse(c, Fail(code, msg))
}
