package utils

import (
	"bytes"
	"crypto/tls"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// FiberCtxToHttpRequest 将fiber.Ctx转换为http.Request
// 尽可能完整地保留原始请求的所有属性
func FiberCtxToHttpRequest(ctx *fiber.Ctx) (*http.Request, error) {
	// 创建一个新的http.Request
	method := string(ctx.Method())
	urlStr := ctx.OriginalURL()

	// 处理请求体
	var body io.Reader
	if len(ctx.Body()) > 0 {
		body = bytes.NewReader(ctx.Body())
	}

	// 解析URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	// 创建请求对象
	req, err := http.NewRequest(method, urlStr, body)
	if err != nil {
		return nil, err
	}

	// 设置URL
	req.URL = parsedURL

	// 设置请求头
	ctx.Request().Header.VisitAll(func(key, value []byte) {
		req.Header.Add(string(key), string(value))
	})

	// 设置协议版本
	proto := string(ctx.Protocol())
	protoMajor, protoMinor := 1, 1 // 默认HTTP/1.1
	if proto == "HTTP/2.0" {
		protoMajor, protoMinor = 2, 0
	} else if proto == "HTTP/1.0" {
		protoMajor, protoMinor = 1, 0
	}
	req.ProtoMajor = protoMajor
	req.ProtoMinor = protoMinor
	req.Proto = proto

	// 设置RemoteAddr
	req.RemoteAddr = ctx.IP()

	// 设置Host
	req.Host = string(ctx.Hostname())

	// 设置ContentLength
	if body != nil {
		req.ContentLength = int64(len(ctx.Body()))
	}

	// 处理表单数据
	if strings.HasPrefix(string(ctx.Get("Content-Type")), "application/x-www-form-urlencoded") {
		form := make(url.Values)
		ctx.Request().PostArgs().VisitAll(func(key, value []byte) {
			form.Add(string(key), string(value))
		})
		req.Form = form
		req.PostForm = form
	}

	// 处理多部分表单数据
	if strings.HasPrefix(string(ctx.Get("Content-Type")), "multipart/form-data") {
		// 注意：由于fiber和标准库处理文件上传的方式不同，这里只能做有限的转换
		// 如果需要处理上传的文件，可能需要额外的代码
		form := make(url.Values)
		ctx.Request().PostArgs().VisitAll(func(key, value []byte) {
			form.Add(string(key), string(value))
		})
		req.Form = form
		req.PostForm = form
		// MultipartForm 需要特殊处理，这里简化处理
	}

	// 处理URL查询参数
	query := make(url.Values)
	ctx.Request().URI().QueryArgs().VisitAll(func(key, value []byte) {
		query.Add(string(key), string(value))
	})
	req.URL.RawQuery = query.Encode()

	// 设置TLS
	if ctx.Secure() {
		req.TLS = &tls.ConnectionState{} // 简化处理，实际情况可能需要更详细的TLS信息
	}

	// 设置Context
	req = req.WithContext(ctx.Context())

	// 设置Cookies
	ctx.Request().Header.VisitAllCookie(func(key, value []byte) {
		cookie := &http.Cookie{
			Name:  string(key),
			Value: string(value),
			Raw:   string(key) + "=" + string(value),
		}
		req.AddCookie(cookie)
	})

	return req, nil
}
