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
	downloadClient *oss.Client
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

// ImageProcessOptions 图片处理选项
type ImageProcessOptions struct {
	// Width 图片宽度（像素），0表示不限制
	Width int
	// Height 图片高度（像素），0表示不限制
	Height int
	// Quality 图片质量（1-100），0表示使用默认质量
	Quality int
	// Format 图片格式（jpg, png, webp等），空表示不转换
	Format string
	// Mode 缩放模式：lfit(默认-等比缩放), mfit(等比缩放填充), fill(固定宽高缩放), pad(等比缩放居中), fixed(固定宽高)
	Mode string
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
		WithRegion(config.Region).
		WithUseInternalEndpoint(true)

	// 创建下载专用客户端配置（不使用自定义域名）
	downloadCfg := oss.LoadDefaultConfig().
		WithCredentialsProvider(provider).
		WithRegion(config.Region)

	// 创建客户端
	client := oss.NewClient(cfg)

	// 返回服务实例
	return &AliyunService{
		config:         config,
		client:         client,
		internalClient: oss.NewClient(internalCfg),
		downloadClient: oss.NewClient(downloadCfg),
		log:            log,
		err:            errBuilder,
		provider:       provider,
	}, nil
}

// dataClient 根据配置选择用于数据操作（下载/上传/删除）的客户端
func (s *AliyunService) dataClient() *oss.Client {
	if s.config.UseInternalDownload {
		return s.internalClient
	}
	return s.client
}

// isVideoObjectKey 判断对象Key是否为视频文件（按扩展名）
func isVideoObjectKey(objectKey string) bool {
	lowerKey := strings.ToLower(objectKey)
	videoExts := []string{".mp4", ".mov", ".m3u8", ".flv", ".avi", ".mkv", ".webm"}
	for _, ext := range videoExts {
		if strings.HasSuffix(lowerKey, ext) {
			return true
		}
	}
	return false
}

// presignClient 根据对象类型和内网配置选择用于生成预签名URL的客户端
func (s *AliyunService) presignClient(objectKey string) *oss.Client {
	isVideo := isVideoObjectKey(objectKey)
	// 视频 + 非内网 -> 使用下载客户端（不走CNAME）
	if isVideo && !s.config.UseInternalDownload {
		return s.downloadClient
	}
	// 非视频 + 内网 -> 使用内网客户端
	if !isVideo && s.config.UseInternalDownload {
		return s.internalClient
	}
	// 其他情况 -> 使用默认客户端
	return s.client
}

// downloadDataClient 根据对象类型和内网配置选择用于直接下载的客户端
func (s *AliyunService) downloadDataClient(objectKey string) *oss.Client {
	isVideo := isVideoObjectKey(objectKey)
	// 视频 + 非内网 -> 使用下载客户端（不走CNAME）
	if isVideo && !s.config.UseInternalDownload {
		return s.downloadClient
	}
	// 其他情况 -> 保持原有dataClient逻辑
	return s.dataClient()
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
		s.log.WithTrace(ctx).WithErr(err).Error("callback json序列化失败")
		return nil, s.err.New("callback json序列化失败", err).WithTraceID(ctx).ToLog(s.log.Entry)
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
		s.log.WithTrace(ctx).WithErr(err).Error("policy json序列化失败")
		return nil, s.err.New("policy json序列化失败", err).WithTraceID(ctx).ToLog(s.log.Entry)
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
		Host:      fmt.Sprintf("https://%s.oss-%s.aliyuncs.com", s.config.Bucket, s.config.Region), // 返回 OSS 上传地址
		Key:       policy.Key,
		Callback:  callbackBase64, // 返回上传回调参数
		//AccessKeyID: cred.AccessKeyID,
	}

	return policyToken, nil
}

// GetPreviewUrl 获取预览URL（支持图片处理参数以节省流量）
// objectKey: 对象键值
// opts: 图片处理选项（可选），如果为nil则返回原图
// expire: URL过期时间
func (s *AliyunService) GetPreviewUrl(ctx context.Context, objectKey string, opts *ImageProcessOptions, expire time.Duration) (string, error) {
	s.log.WithTrace(ctx).WithField("objectKey", objectKey).WithField("opts", opts).Debug("获取阿里云文件预览URL")

	// 保证objectKey不以"/"开头
	objectKey = s.ValidAndProcessOssKey(objectKey)

	// 设置过期时间
	if expire <= 0 {
		expire = 5 * time.Second
	}

	// 创建GET请求对象
	request := &oss.GetObjectRequest{
		Bucket: oss.Ptr(s.config.Bucket),
		Key:    oss.Ptr(objectKey),
	}

	// 如果提供了图片处理选项，构造处理参数
	if opts != nil {
		processParam := s.buildImageProcessParam(opts)
		if processParam != "" {
			request.Process = oss.Ptr(processParam)
		}
	}

	// 根据文件类型和内网配置选择合适的客户端
	client := s.presignClient(objectKey)
	result, err := client.Presign(ctx, request,
		oss.PresignExpires(expire),
	)
	if err != nil {
		return "", s.err.New("生成预签名预览URL失败", err).WithTraceID(ctx).ToLog(s.log.Entry)
	}

	s.log.WithTrace(ctx).WithField("previewUrl", result.URL).Debug("成功生成预览URL")
	return result.URL, nil
}

// GetPreviewUrlSimple 获取预览URL的简化版本（只指定宽高）
// 使用默认的等比缩放模式(lfit)和默认质量
func (s *AliyunService) GetPreviewUrlSimple(ctx context.Context, objectKey string, width, height int, expire time.Duration) (string, error) {
	opts := &ImageProcessOptions{
		Width:  width,
		Height: height,
		Mode:   "lfit",
	}
	return s.GetPreviewUrl(ctx, objectKey, opts, expire)
}

// GetPreviewUrlWithQuality 获取预览URL并指定质量
// 用于在保持尺寸的同时进一步压缩图片
func (s *AliyunService) GetPreviewUrlWithQuality(ctx context.Context, objectKey string, width, height, quality int, expire time.Duration) (string, error) {
	opts := &ImageProcessOptions{
		Width:   width,
		Height:  height,
		Quality: quality,
		Mode:    "lfit",
	}
	return s.GetPreviewUrl(ctx, objectKey, opts, expire)
}

// GetPreviewUrlWebP 获取WebP格式的预览URL
// WebP格式通常能提供更好的压缩率
func (s *AliyunService) GetPreviewUrlWebP(ctx context.Context, objectKey string, width, height, quality int, expire time.Duration) (string, error) {
	opts := &ImageProcessOptions{
		Width:   width,
		Height:  height,
		Quality: quality,
		Format:  "webp",
		Mode:    "lfit",
	}
	return s.GetPreviewUrl(ctx, objectKey, opts, expire)
}

// buildImageProcessParam 构建图片处理参数
// 阿里云OSS图片处理格式：image/操作1,参数/操作2,参数/操作3,参数
// 例如：image/resize,m_lfit,w_100,h_100/quality,q_80/format,webp
func (s *AliyunService) buildImageProcessParam(opts *ImageProcessOptions) string {
	if opts == nil {
		return ""
	}

	var operations []string

	// 如果指定了宽度或高度，添加缩放参数
	if opts.Width > 0 || opts.Height > 0 {
		mode := opts.Mode
		if mode == "" {
			mode = "lfit" // 默认等比缩放限制在指定w与h的矩形内
		}

		resizeParam := fmt.Sprintf("resize,m_%s", mode)
		if opts.Width > 0 {
			resizeParam += fmt.Sprintf(",w_%d", opts.Width)
		}
		if opts.Height > 0 {
			resizeParam += fmt.Sprintf(",h_%d", opts.Height)
		}
		operations = append(operations, resizeParam)
	}

	// 如果指定了质量，添加质量参数
	if opts.Quality > 0 && opts.Quality <= 100 {
		operations = append(operations, fmt.Sprintf("quality,q_%d", opts.Quality))
	}

	// 如果指定了格式转换，添加格式转换参数
	if opts.Format != "" {
		operations = append(operations, fmt.Sprintf("format,%s", opts.Format))
	}

	// 使用 / 连接多个处理操作，并在开头加上 image/
	if len(operations) > 0 {
		return "image/" + strings.Join(operations, "/")
	}

	return ""
}

// GetDownloadUrl 获取下载URL
func (s *AliyunService) GetDownloadUrl(ctx context.Context, objectKey string, name string, speedLimit int64, expire time.Duration) (string, error) {
	s.log.WithTrace(ctx).WithField("objectKey", objectKey).Info("获取阿里云文件下载URL")

	// 保证objectKey不以"/"开头
	objectKey = s.ValidAndProcessOssKey(objectKey)

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

	// 根据文件类型和内网配置选择合适的客户端
	client := s.presignClient(objectKey)
	result, err := client.Presign(ctx, request,
		oss.PresignExpires(expire),
	)
	if err != nil {
		return "", s.err.New("生成预签名下载URL失败", err).WithTraceID(ctx).ToLog(s.log.Entry)
	}
	return result.URL, nil
}

// DownloadFile 直接下载文件内容
func (s *AliyunService) DownloadFile(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	s.log.WithTrace(ctx).WithField("objectKey", objectKey).Info("直接下载阿里云文件")

	// 保证objectKey不以"/"开头
	objectKey = s.ValidAndProcessOssKey(objectKey)

	// 创建获取对象的请求
	request := &oss.GetObjectRequest{
		Bucket: oss.Ptr(s.config.Bucket),
		Key:    oss.Ptr(objectKey),
	}

	// 根据文件类型和内网配置选择合适的客户端
	client := s.downloadDataClient(objectKey)
	result, err := client.GetObject(ctx, request)
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
	objectKey = s.ValidAndProcessOssKey(objectKey)

	// 创建删除对象的请求
	request := &oss.DeleteObjectRequest{
		Bucket: oss.Ptr(s.config.Bucket),
		Key:    oss.Ptr(objectKey),
	}

	// 执行删除请求
	_, err := s.dataClient().DeleteObject(ctx, request)
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

	objectKey = s.ValidAndProcessOssKey(objectKey)

	// 创建上传对象的请求
	request := &oss.PutObjectRequest{
		Bucket: oss.Ptr(s.config.Bucket),
		Key:    oss.Ptr(objectKey),
		Body:   reader,
	}

	// 执行上传请求
	_, err := s.dataClient().PutObject(ctx, request)
	if err != nil {
		return s.err.New("上传文件到阿里云OSS失败", err).WithTraceID(ctx).ToLog(s.log.Entry)
	}

	return nil
}

// AppendFile 向已存在或可创建的追加写对象追加一段内容（position 为 0 时创建追加对象）
// 返回下一次追加应使用的 position（即当前对象总长度）
func (s *AliyunService) AppendFile(ctx context.Context, objectKey string, reader io.Reader, position int64) (int64, error) {
	s.log.WithTrace(ctx).WithField("objectKey", objectKey).WithField("position", position).Debug("追加文件到阿里云OSS")

	objectKey = s.ValidAndProcessOssKey(objectKey)

	req := &oss.AppendObjectRequest{
		Bucket:   oss.Ptr(s.config.Bucket),
		Key:      oss.Ptr(objectKey),
		Position: oss.Ptr(position),
		Body:     reader,
	}
	if position == 0 {
		req.InitHashCRC64 = oss.Ptr("0")
	}

	result, err := s.dataClient().AppendObject(ctx, req)
	if err != nil {
		return position, s.err.New("追加文件到阿里云OSS失败", err).WithTraceID(ctx).ToLog(s.log.Entry)
	}
	if result == nil {
		return position, nil
	}
	return result.NextPosition, nil
}

// GetThumbnailUrl 获取图片缩略图URL
func (s *AliyunService) GetThumbnailUrl(ctx context.Context, objectKey string, width, height int, expire time.Duration) (string, error) {
	s.log.WithTrace(ctx).WithField("objectKey", objectKey).WithField("width", width).WithField("height", height).Info("获取图片缩略图URL")

	// 保证objectKey不以"/"开头
	objectKey = s.ValidAndProcessOssKey(objectKey)

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
	objectKey = s.ValidAndProcessOssKey(objectKey)

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

func (s *AliyunService) ValidAndProcessOssKey(objectKey string) string {
	// 保证objectKey不以"/"开头
	objectKey = strings.TrimPrefix(objectKey, "/")
	objectKey = strings.TrimSuffix(objectKey, "/")
	return objectKey
}
