package oss

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"strings"
	"time"
	"github.com/xsxdot/aio/pkg/core/config"
	errorc "github.com/xsxdot/aio/pkg/core/err"
	"github.com/xsxdot/aio/pkg/core/logger"

	"github.com/gofiber/fiber/v2"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
)

// AliyunService 阿里云OSS服务实现
type AliyunService struct {
	config         *config.OssConfig
	client         *oss.Client
	internalClient *oss.Client
	log            *logger.Log
	err            *errorc.ErrorBuilder
	provider       credentials.CredentialsProvider
}

type PolicyToken struct {
	Policy string `json:"policy"`
	//SecurityToken    string `json:"x-oss-security-token"`
	SignatureVersion string `json:"x-oss-signature-version"`
	Credential       string `json:"x-oss-credential"`
	Date             string `json:"x-oss-date"`
	//SignatureV4      string `json:"x-oss-signature"`
	Signature string `json:"x-oss-signature"`
	Acl       string `json:"x-oss-object-acl"`
	Host      string `json:"host"`
	Key       string `json:"key"`
	Callback  string `json:"callback"`
	//AccessKeyID string `json:"OSSAccessKeyId"`
}

type CallbackParam struct {
	CallbackUrl      string `json:"callbackUrl"`
	CallbackBody     string `json:"callbackBody"`
	CallbackBodyType string `json:"callbackBodyType"`
}

// NewAliyunService 创建阿里云OSS服务实例
func NewAliyunService(config *config.OssConfig) (*AliyunService, error) {
	log := logger.GetLogger().WithEntryName("AliyunOSSService")
	errBuilder := errorc.NewErrorBuilder("AliyunOSSService")

	if config.AccessKeyID == "" || config.AccessKeySecret == "" || config.Bucket == "" {
		return nil, errBuilder.New("阿里云配置不完整", nil).ValidWithCtx().ToLog(log.Entry)
	}

	// 创建阿里云OSS客户端配置
	provider := credentials.NewStaticCredentialsProvider(config.AccessKeyID, config.AccessKeySecret, "")
	cfg := oss.LoadDefaultConfig().
		WithCredentialsProvider(provider).
		WithRegion(config.Region)

	if config.Domain != "" {
		cfg = cfg.WithEndpoint(config.Domain).WithUseCName(true)
	}

	internalCfg := oss.LoadDefaultConfig().
		WithCredentialsProvider(provider).
		WithRegion(config.Region)

	// 创建客户端
	client := oss.NewClient(cfg)

	// 返回服务实例
	return &AliyunService{
		config:         config,
		client:         client,
		internalClient: oss.NewClient(internalCfg),
		log:            log,
		err:            errBuilder,
		provider:       provider,
	}, nil
}

// GetUploadToken 获取上传令牌
func (s *AliyunService) GetUploadToken(ctx context.Context, policy *UploadPolicy) (interface{}, error) {
	s.log.WithTrace(ctx).WithField("policy", policy).Info("获取阿里云上传令牌")

	cred, err := s.provider.GetCredentials(ctx)
	if err != nil {
		return "", err
	}

	var callbackParam CallbackParam
	callbackParam.CallbackUrl = policy.CallbackUrl
	callbackParam.CallbackBody = "{\"mimeType\":${mimeType},\"size\":${size},\"key\":${object},\"hash\":${etag}}"
	callbackParam.CallbackBodyType = "application/json"
	callback_str, err := json.Marshal(callbackParam)
	if err != nil {
		fmt.Println("callback json err:", err)
	}
	callbackBase64 := base64.StdEncoding.EncodeToString(callback_str)

	acl := "default"
	if policy.IsPublic {
		acl = "public-read"
	}

	// 创建阿里云OSS上传策略
	utcTime := time.Now().UTC()
	date := utcTime.Format("20060102")
	expiration := utcTime.Add(1 * time.Hour)
	credential := fmt.Sprintf("%v/%v/%v/%v/aliyun_v4_request",
		cred.AccessKeyID, date, s.config.Region, "oss")
	policyMap := map[string]any{
		"expiration": expiration.Format("2006-01-02T15:04:05.000Z"),
		"conditions": []any{
			map[string]string{"bucket": s.config.Bucket},
			map[string]string{"x-oss-object-acl": acl},
			map[string]string{"callback": callbackBase64},
			map[string]string{"x-oss-signature-version": "OSS4-HMAC-SHA256"},
			map[string]string{"x-oss-credential": credential}, // 凭证
			map[string]string{"x-oss-date": utcTime.Format("20060102T150405Z")},
			// 其他条件
			[]any{"content-length-range", 1, policy.MaxSize},
			[]any{"eq", "$key", policy.Key},
			// []any{"in", "$content-type", []string{"image/jpg", "image/png"}},
		},
	}

	// 将policy转换为 JSON 格式
	policyBody, err := json.Marshal(policyMap)
	if err != nil {
		s.log.Fatalf("json.Marshal fail, err:%v", err)
	}

	// 构造待签名字符串（StringToSign）
	stringToSign := base64.StdEncoding.EncodeToString(policyBody)

	hmacHash := func() hash.Hash { return sha256.New() }
	// 构建signing key
	signingKey := "aliyun_v4" + cred.AccessKeySecret
	h1 := hmac.New(hmacHash, []byte(signingKey))
	io.WriteString(h1, date)
	h1Key := h1.Sum(nil)

	h2 := hmac.New(hmacHash, h1Key)
	io.WriteString(h2, s.config.Region)
	h2Key := h2.Sum(nil)

	h3 := hmac.New(hmacHash, h2Key)
	io.WriteString(h3, "oss")
	h3Key := h3.Sum(nil)

	h4 := hmac.New(hmacHash, h3Key)
	io.WriteString(h4, "aliyun_v4_request")
	h4Key := h4.Sum(nil)

	// 生成签名
	h := hmac.New(hmacHash, h4Key)
	io.WriteString(h, stringToSign)
	signature := hex.EncodeToString(h.Sum(nil))

	// 构建返回给前端的表单
	policyToken := PolicyToken{
		Policy: stringToSign,
		//SecurityToken:    cred.SecurityToken,
		SignatureVersion: "OSS4-HMAC-SHA256",
		Credential:       credential,
		Date:             utcTime.UTC().Format("20060102T150405Z"),
		//SignatureV4:      signature,
		Signature: signature,
		Acl:       acl,
		Host:      fmt.Sprintf("http://%s.oss-%s.aliyuncs.com", s.config.Bucket, s.config.Region), // 返回 OSS 上传地址
		Key:       policy.Key,
		Callback:  callbackBase64, // 返回上传回调参数
		//AccessKeyID: cred.AccessKeyID,
	}

	return policyToken, nil
}

// GetPreviewUrl 获取预览URL
func (s *AliyunService) GetPreviewUrl(ctx context.Context, objectKey string, expireDay time.Duration) (string, error) {
	return s.GetDownloadUrl(ctx, objectKey, "", 0, expireDay)
}

// GetDownloadUrl 获取下载URL
func (s *AliyunService) GetDownloadUrl(ctx context.Context, objectKey string, name string, speedLimit int64, expire time.Duration) (string, error) {
	s.log.WithTrace(ctx).WithField("objectKey", objectKey).Info("获取阿里云文件下载URL")

	// 保证objectKey不以"/"开头
	if strings.HasPrefix(objectKey, "/") {
		objectKey = objectKey[1:]
	}

	// 设置过期时间
	if expire <= 0 {
		expire = 5 * time.Second
	}

	request := &oss.GetObjectRequest{
		Bucket: oss.Ptr(s.config.Bucket),
		Key:    oss.Ptr(objectKey),
	}
	if speedLimit > 0 {
		request.TrafficLimit = speedLimit
	}
	if name != "" {
		request.ResponseContentDisposition = oss.Ptr(fmt.Sprintf("attachment;filename=%s", name))
	}
	result, err := s.client.Presign(context.TODO(), request,
		oss.PresignExpires(expire),
	)
	if err != nil {
		s.log.Fatalf("failed to get object presign %v", err)
	}
	return result.URL, nil
}

// DownloadFile 直接下载文件内容
func (s *AliyunService) DownloadFile(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	s.log.WithTrace(ctx).WithField("objectKey", objectKey).Info("直接下载阿里云文件")

	// 保证objectKey不以"/"开头
	if strings.HasPrefix(objectKey, "/") {
		objectKey = objectKey[1:]
	}

	// 创建获取对象的请求
	request := &oss.GetObjectRequest{
		Bucket: oss.Ptr(s.config.Bucket),
		Key:    oss.Ptr(objectKey),
	}

	// 执行下载请求
	result, err := s.client.GetObject(ctx, request)
	if err != nil {
		return nil, s.err.New("直接下载阿里云文件失败", err).WithTraceID(ctx).ToLog(s.log.Entry)
	}

	s.log.WithTrace(ctx).Info("成功下载阿里云文件")
	return result.Body, nil
}

// DeleteFile 直接删除文件
func (s *AliyunService) DeleteFile(ctx context.Context, objectKey string) error {
	s.log.WithTrace(ctx).WithField("objectKey", objectKey).Info("直接删除阿里云文件")

	//.保证objectKey不以"/"开头
	if strings.HasPrefix(objectKey, "/") {
		objectKey = objectKey[1:]
	}

	// 创建删除对象的请求
	request := &oss.DeleteObjectRequest{
		Bucket: oss.Ptr(s.config.Bucket),
		Key:    oss.Ptr(objectKey),
	}

	// 执行删除请求
	_, err := s.client.DeleteObject(ctx, request)
	if err != nil {
		return s.err.New("删除阿里云文件失败", err).WithTraceID(ctx).ToLog(s.log.Entry)
	}

	s.log.WithTrace(ctx).Info("已成功删除阿里云文件")

	return nil
}

// ValidCallback 验证回调
func (s *AliyunService) ValidCallback(ctx context.Context, r *fiber.Ctx) bool {
	s.log.WithTrace(ctx).Info("验证阿里云回调")

	// Get PublicKey bytes
	bytePublicKey, err := getPublicKey(r)
	if err != nil {
		return false
	}

	// Get Authorization bytes : decode from Base64String
	byteAuthorization, err := getAuthorization(r)
	if err != nil {
		return false
	}

	// Get MD5 bytes from Newly Constructed Authrization String.
	byteMD5, err := getMD5FromNewAuthString(r)
	if err != nil {
		return false
	}

	// VerifySignature and response to client
	return verifySignature(bytePublicKey, byteMD5, byteAuthorization)
}

// UploadFile 上传文件
func (s *AliyunService) UploadFile(ctx context.Context, objectKey string, reader io.Reader) error {
	s.log.WithTrace(ctx).WithField("objectKey", objectKey).Info("上传文件到阿里云OSS")

	// 保证objectKey不以"/"开头
	if strings.HasPrefix(objectKey, "/") {
		objectKey = objectKey[1:]
	}

	// 创建上传对象的请求
	request := &oss.PutObjectRequest{
		Bucket: oss.Ptr(s.config.Bucket),
		Key:    oss.Ptr(objectKey),
		Body:   reader,
	}

	// 执行上传请求
	_, err := s.client.PutObject(ctx, request)
	if err != nil {
		return s.err.New("上传文件到阿里云OSS失败", err).WithTraceID(ctx).ToLog(s.log.Entry)
	}

	return nil
}

// GetThumbnailUrl 获取图片缩略图URL
func (s *AliyunService) GetThumbnailUrl(ctx context.Context, objectKey string, width, height int, expire time.Duration) (string, error) {
	s.log.WithTrace(ctx).WithField("objectKey", objectKey).WithField("width", width).WithField("height", height).Info("获取图片缩略图URL")

	// 保证objectKey不以"/"开头
	if strings.HasPrefix(objectKey, "/") {
		objectKey = objectKey[1:]
	}

	// 设置过期时间，默认1小时
	if expire <= 0 {
		expire = 1 * time.Hour
	}

	// 参数验证
	if width <= 0 || height <= 0 {
		return "", s.err.New("缩略图宽度和高度必须大于0", nil).WithTraceID(ctx).ToLog(s.log.Entry)
	}

	// 构造图片缩放处理参数 - 使用固定宽高模式
	processParam := fmt.Sprintf("image/resize,m_fixed,w_%d,h_%d", width, height)

	// 创建GET请求对象
	request := &oss.GetObjectRequest{
		Bucket:  oss.Ptr(s.config.Bucket),
		Key:     oss.Ptr(objectKey),
		Process: oss.Ptr(processParam),
	}

	// 生成带签名的预签名URL
	result, err := s.client.Presign(ctx, request,
		oss.PresignExpires(expire),
	)
	if err != nil {
		return "", s.err.New("生成图片缩略图URL失败", err).WithTraceID(ctx).ToLog(s.log.Entry)
	}

	s.log.WithTrace(ctx).WithField("thumbnailUrl", result.URL).Info("成功生成图片缩略图URL")

	return result.URL, nil
}

// GetVideoCoverUrl 获取视频封面URL
func (s *AliyunService) GetVideoCoverUrl(ctx context.Context, objectKey string, timeSeconds int, expire time.Duration) (string, error) {
	s.log.WithTrace(ctx).WithField("objectKey", objectKey).WithField("timeSeconds", timeSeconds).Info("获取视频封面URL")

	// 保证objectKey不以"/"开头
	if strings.HasPrefix(objectKey, "/") {
		objectKey = objectKey[1:]
	}

	// 设置过期时间，默认1小时
	if expire <= 0 {
		expire = 1 * time.Hour
	}

	// 如果时间参数小于0，设置为0
	if timeSeconds < 0 {
		timeSeconds = 0
	}

	// 构造视频截帧处理参数
	processParam := fmt.Sprintf("video/snapshot,t_%d", timeSeconds)

	// 创建GET请求对象
	request := &oss.GetObjectRequest{
		Bucket:  oss.Ptr(s.config.Bucket),
		Key:     oss.Ptr(objectKey),
		Process: oss.Ptr(processParam),
	}

	// 生成带签名的预签名URL
	result, err := s.client.Presign(ctx, request,
		oss.PresignExpires(expire),
	)
	if err != nil {
		return "", s.err.New("生成视频封面URL失败", err).WithTraceID(ctx).ToLog(s.log.Entry)
	}

	s.log.WithTrace(ctx).WithField("coverUrl", result.URL).Info("成功生成视频封面URL")

	return result.URL, nil
}
