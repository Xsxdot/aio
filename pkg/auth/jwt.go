package auth

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	common2 "github.com/xsxdot/aio/pkg/common"

	"github.com/golang-jwt/jwt/v5"
)

// JWTClaims JWT声明
type JWTClaims struct {
	jwt.RegisteredClaims
	SubjectID   string                 `json:"subject_id"`
	SubjectType string                 `json:"subject_type"`
	Name        string                 `json:"name"`
	Roles       []string               `json:"roles,omitempty"`
	Permissions []Permission           `json:"permissions,omitempty"`
	Extra       map[string]interface{} `json:"extra,omitempty"`
}

// JWTConfig JWT配置
type JWTConfig struct {
	// 私钥文件路径
	PrivateKeyPath string `yaml:"private_key_path"`
	// 公钥文件路径
	PublicKeyPath string `yaml:"public_key_path"`
	// 访问令牌过期时间
	AccessTokenExpiry time.Duration `yaml:"access_token_expiry"`
	// 发行者
	Issuer string `yaml:"issuer"`
	// 受众
	Audience string `yaml:"audience"`
}

// JWTService JWT服务
type JWTService struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	config     JWTConfig
	logger     *common2.Logger
}

// NewJWTService 创建JWT服务
func NewJWTService(config JWTConfig) (*JWTService, error) {
	service := &JWTService{
		config: config,
		logger: common2.GetLogger().WithField("component", "jwt_service"),
	}

	// 加载私钥（如果提供）
	if config.PrivateKeyPath != "" {
		privateKey, err := LoadRSAPrivateKeyFromFile(config.PrivateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("加载私钥失败: %w", err)
		}
		service.privateKey = privateKey
	}

	// 加载公钥（如果提供）
	if config.PublicKeyPath != "" {
		publicKey, err := LoadRSAPublicKeyFromFile(config.PublicKeyPath)
		if err != nil {
			return nil, fmt.Errorf("加载公钥失败: %w", err)
		}
		service.publicKey = publicKey
	}

	return service, nil
}

// NewJWTServiceFromKeys 从RSA密钥创建JWT服务
func NewJWTServiceFromKeys(privateKey *rsa.PrivateKey, publicKey *rsa.PublicKey, config JWTConfig) *JWTService {
	return &JWTService{
		privateKey: privateKey,
		publicKey:  publicKey,
		config:     config,
		logger:     common2.GetLogger().WithField("component", "jwt_service"),
	}
}

// GenerateToken 生成JWT令牌
func (s *JWTService) GenerateToken(authInfo AuthInfo) (*Token, error) {
	if s.privateKey == nil {
		return nil, errors.New("未配置私钥，无法生成令牌")
	}

	now := time.Now()
	expiry := s.config.AccessTokenExpiry
	if expiry == 0 {
		expiry = 1 * time.Hour // 默认1小时过期
	}

	// 创建JWT声明
	claims := JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    s.config.Issuer,
			Subject:   authInfo.SubjectID,
			Audience:  []string{s.config.Audience},
		},
		SubjectID:   authInfo.SubjectID,
		SubjectType: string(authInfo.SubjectType),
		Name:        authInfo.Name,
		Roles:       authInfo.Roles,
		Permissions: authInfo.Permissions,
		Extra:       authInfo.Extra,
	}

	// 创建令牌
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	// 签名令牌
	tokenString, err := token.SignedString(s.privateKey)
	if err != nil {
		return nil, fmt.Errorf("签名令牌失败: %w", err)
	}

	// 创建令牌响应
	tokenResponse := &Token{
		AccessToken: tokenString,
		ExpiresIn:   int64(expiry.Seconds()),
		TokenType:   "Bearer",
	}

	return tokenResponse, nil
}

// ValidateToken 验证JWT令牌
func (s *JWTService) ValidateToken(tokenString string) (*AuthInfo, error) {
	if s.publicKey == nil {
		return nil, errors.New("未配置公钥，无法验证令牌")
	}

	// 解析令牌
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// 验证签名算法
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("非预期的签名算法: %v", token.Header["alg"])
		}
		return s.publicKey, nil
	})

	if err != nil {
		return nil, common2.NewUnauthorizedError("令牌验证失败", err)
	}

	// 检查令牌有效性
	if !token.Valid {
		return nil, common2.NewUnauthorizedError("无效的令牌", nil)
	}

	// 提取声明
	claims, ok := token.Claims.(*JWTClaims)
	if !ok {
		return nil, common2.NewUnauthorizedError("无效的令牌声明", nil)
	}

	// 创建认证信息
	authInfo := &AuthInfo{
		SubjectID:   claims.SubjectID,
		SubjectType: SubjectType(claims.SubjectType),
		Name:        claims.Name,
		Roles:       claims.Roles,
		Permissions: claims.Permissions,
		Extra:       claims.Extra,
	}

	return authInfo, nil
}

// ExtractClaims 从令牌中提取声明
func (s *JWTService) ExtractClaims(tokenString string) (*JWTClaims, error) {
	if s.publicKey == nil {
		return nil, errors.New("未配置公钥，无法提取令牌声明")
	}

	// 解析令牌
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// 验证签名算法
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("非预期的签名算法: %v", token.Header["alg"])
		}
		return s.publicKey, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			// 即使令牌过期，我们也可能需要提取声明
			if token != nil {
				if claims, ok := token.Claims.(*JWTClaims); ok {
					return claims, jwt.ErrTokenExpired
				}
			}
		}
		return nil, err
	}

	// 提取声明
	claims, ok := token.Claims.(*JWTClaims)
	if !ok {
		return nil, fmt.Errorf("无效的令牌声明")
	}

	return claims, nil
}

// GenerateRSAKeyPair 生成RSA密钥对
func GenerateRSAKeyPair(bits int) (*RSAKeyPair, error) {
	// 生成RSA密钥对
	privateKey, err := GenerateRSAKey(bits)
	if err != nil {
		return nil, fmt.Errorf("生成RSA密钥对失败: %w", err)
	}

	// 编码私钥
	privateKeyPEM, err := EncodeRSAPrivateKeyToPEM(privateKey)
	if err != nil {
		return nil, fmt.Errorf("编码私钥失败: %w", err)
	}

	// 编码公钥
	publicKeyPEM, err := EncodeRSAPublicKeyToPEM(&privateKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("编码公钥失败: %w", err)
	}

	// 创建RSA密钥对
	keyPair := &RSAKeyPair{
		PrivateKey:    privateKey,
		PrivateKeyPEM: string(privateKeyPEM),
		PublicKey:     &privateKey.PublicKey,
		PublicKeyPEM:  string(publicKeyPEM),
	}

	return keyPair, nil
}

// LoadRSAKeyPair 从PEM字符串加载RSA密钥对
func LoadRSAKeyPair(privateKeyPEM, publicKeyPEM string) (*RSAKeyPair, error) {
	// 解码私钥
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		return nil, errors.New("无效的私钥PEM格式")
	}
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("解析私钥失败: %w", err)
	}

	// 解码公钥
	block, _ = pem.Decode([]byte(publicKeyPEM))
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, errors.New("无效的公钥PEM格式")
	}
	publicKeyInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("解析公钥失败: %w", err)
	}
	publicKey, ok := publicKeyInterface.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("不是有效的RSA公钥")
	}

	// 创建RSA密钥对
	keyPair := &RSAKeyPair{
		PrivateKey:    privateKey,
		PrivateKeyPEM: privateKeyPEM,
		PublicKey:     publicKey,
		PublicKeyPEM:  publicKeyPEM,
	}

	return keyPair, nil
}

// InitializeJWTConfig 初始化 JWT 配置
func InitializeJWTConfig(dataDir string, isMaster bool) (*AuthJWTConfig, error) {
	certDir := filepath.Join(dataDir, "cert")

	// 确保证书目录存在
	if err := os.MkdirAll(certDir, 0755); err != nil {
		return nil, fmt.Errorf("创建证书目录失败: %w", err)
	}

	jwtConfig := &AuthJWTConfig{
		PrivateKeyPath:    filepath.Join(certDir, "jwt.key"),
		PublicKeyPath:     filepath.Join(certDir, "jwt.key.pub"),
		AccessTokenExpiry: 48 * time.Hour,
		Issuer:            "aio-system",
		Audience:          "aio-api",
	}

	var keyPair *RSAKeyPair
	var err error

	if _, statErr := os.Stat(jwtConfig.PrivateKeyPath); statErr != nil {
		if !isMaster {
			return nil, fmt.Errorf("master节点才能创建jwt密钥")
		}
		keyPair, err = GenerateAndSaveRSAKeyPair(2048, jwtConfig.PrivateKeyPath, jwtConfig.PublicKeyPath)
		if err != nil {
			return nil, fmt.Errorf("生成JWT密钥失败: %w", err)
		}
	} else {
		keyPair, err = LoadRSAKeyPairFromFiles(jwtConfig.PrivateKeyPath, jwtConfig.PublicKeyPath)
		if err != nil {
			return nil, fmt.Errorf("加载JWT密钥失败: %w", err)
		}
	}

	jwtConfig.KeyPair = *keyPair

	// 设置全局配置
	GlobalJWTConfig = jwtConfig

	return jwtConfig, nil
}
