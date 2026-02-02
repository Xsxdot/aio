package service

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/system/ssl/internal/model"

	cas "github.com/alibabacloud-go/cas-20200407/v2/client"
	cdn "github.com/alibabacloud-go/cdn-20180510/v4/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	dcdn "github.com/alibabacloud-go/dcdn-20180115/v3/client"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// DeployService 部署服务
// 实现本机文件、SSH 远端、阿里云 CAS 三种部署方式
type DeployService struct {
	log       *logger.Log
	err       *errorc.ErrorBuilder
	cryptoSvc *CryptoService
}

// GetBucketCnameResult OSS GetBucketCname 返回的 XML 结构
type GetBucketCnameResult struct {
	XMLName xml.Name          `xml:"ListCnameResult"`
	Bucket  string            `xml:"Bucket"`
	Owner   string            `xml:"Owner"`
	Cnames  []CnameConfigInfo `xml:"Cname"`
}

// CnameConfigInfo CNAME 配置信息
type CnameConfigInfo struct {
	Domain string `xml:"Domain"`
	Status string `xml:"Status"`
}

// NewDeployService 创建部署服务实例
func NewDeployService(log *logger.Log, cryptoSvc *CryptoService) *DeployService {
	return &DeployService{
		log:       log.WithEntryName("DeployService"),
		err:       errorc.NewErrorBuilder("DeployService"),
		cryptoSvc: cryptoSvc,
	}
}

// Deploy 部署证书到目标
// 返回部署结果数据（JSON 字符串）
func (s *DeployService) Deploy(ctx context.Context, target *model.DeployTarget, fullchainPem, privkeyPem, domainsJSON string) (string, error) {
	s.log.WithFields(map[string]interface{}{
		"target_id":   target.ID,
		"target_name": target.Name,
		"target_type": target.Type,
	}).Info("开始部署证书")

	switch target.Type {
	case model.DeployTargetTypeLocal:
		return s.deployToLocal(ctx, target, fullchainPem, privkeyPem, domainsJSON)
	case model.DeployTargetTypeSSH:
		return s.deployToSSH(ctx, target, fullchainPem, privkeyPem, domainsJSON)
	case model.DeployTargetTypeAliyunCAS:
		return s.deployToAliyunCAS(ctx, target, fullchainPem, privkeyPem, domainsJSON)
	default:
		return "", s.err.New("不支持的部署类型", nil).ValidWithCtx()
	}
}

// deployToLocal 部署到本机文件
func (s *DeployService) deployToLocal(ctx context.Context, target *model.DeployTarget, fullchainPem, privkeyPem, domainsJSON string) (string, error) {
	// 1. 解析配置
	var config model.LocalDeployConfig
	if err := json.Unmarshal([]byte(target.Config), &config); err != nil {
		return "", s.err.New("解析本机部署配置失败", err)
	}

	// 2. 确定目标路径（直接使用 BasePath）
	targetPath := config.BasePath

	// 3. 创建目录
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		return "", s.err.New("创建目标目录失败", err)
	}

	// 4. 确定文件名
	fullchainName := config.FullchainName
	if fullchainName == "" {
		fullchainName = "fullchain.pem"
	}
	privkeyName := config.PrivkeyName
	if privkeyName == "" {
		privkeyName = "privkey.pem"
	}

	// 5. 写入证书文件
	fullchainPath := filepath.Join(targetPath, fullchainName)
	privkeyPath := filepath.Join(targetPath, privkeyName)

	fileMode := os.FileMode(0600)
	if config.FileMode != "" {
		if mode, err := strconv.ParseUint(config.FileMode, 8, 32); err == nil {
			fileMode = os.FileMode(mode)
		}
	}

	if err := os.WriteFile(fullchainPath, []byte(fullchainPem), fileMode); err != nil {
		return "", s.err.New("写入 fullchain.pem 失败", err)
	}

	if err := os.WriteFile(privkeyPath, []byte(privkeyPem), fileMode); err != nil {
		return "", s.err.New("写入 privkey.pem 失败", err)
	}

	s.log.WithFields(map[string]interface{}{
		"fullchain_path": fullchainPath,
		"privkey_path":   privkeyPath,
	}).Info("证书文件写入成功")

	// 6. 执行重载命令（可选）
	reloadOutput := ""
	if config.ReloadCommand != "" {
		s.log.WithField("command", config.ReloadCommand).Info("执行重载命令")
		output, err := s.executeCommand(config.ReloadCommand)
		if err != nil {
			s.log.WithErr(err).Warn("执行重载命令失败")
			reloadOutput = fmt.Sprintf("执行失败: %v", err)
		} else {
			reloadOutput = output
		}
	}

	// 7. 返回结果
	result := map[string]interface{}{
		"fullchain_path": fullchainPath,
		"privkey_path":   privkeyPath,
		"reload_output":  reloadOutput,
	}
	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// deployToSSH 部署到 SSH 远端
func (s *DeployService) deployToSSH(ctx context.Context, target *model.DeployTarget, fullchainPem, privkeyPem, domainsJSON string) (string, error) {
	// 1. 解析运行时配置（已由 App 层填充完整）
	var config model.SSHDeployRuntimeConfig
	if err := json.Unmarshal([]byte(target.Config), &config); err != nil {
		return "", s.err.New("解析 SSH 部署配置失败", err)
	}

	// 注意：配置已由 App 层解密，无需再次解密

	// 2. 建立 SSH 连接
	sshClient, err := s.createSSHClient(&config)
	if err != nil {
		return "", s.err.New("建立 SSH 连接失败", err)
	}
	defer sshClient.Close()

	// 3. 创建 SFTP 客户端
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return "", s.err.New("创建 SFTP 客户端失败", err)
	}
	defer sftpClient.Close()

	// 4. 确定目标路径（直接使用 RemotePath）
	targetPath := config.RemotePath

	// 5. 创建远程目录
	if err := sftpClient.MkdirAll(targetPath); err != nil {
		return "", s.err.New("创建远程目录失败", err)
	}

	// 6. 确定文件名
	fullchainName := config.FullchainName
	if fullchainName == "" {
		fullchainName = "fullchain.pem"
	}
	privkeyName := config.PrivkeyName
	if privkeyName == "" {
		privkeyName = "privkey.pem"
	}

	// 7. 上传证书文件
	fullchainPath := filepath.Join(targetPath, fullchainName)
	privkeyPath := filepath.Join(targetPath, privkeyName)

	if err := s.uploadFile(sftpClient, fullchainPath, []byte(fullchainPem), config.FileMode); err != nil {
		return "", s.err.New("上传 fullchain.pem 失败", err)
	}

	if err := s.uploadFile(sftpClient, privkeyPath, []byte(privkeyPem), config.FileMode); err != nil {
		return "", s.err.New("上传 privkey.pem 失败", err)
	}

	s.log.WithFields(map[string]interface{}{
		"fullchain_path": fullchainPath,
		"privkey_path":   privkeyPath,
	}).Info("证书文件上传成功")

	// 8. 执行远程重载命令（可选）
	reloadOutput := ""
	if config.ReloadCommand != "" {
		s.log.WithField("command", config.ReloadCommand).Info("执行远程重载命令")
		output, err := s.executeSSHCommand(sshClient, config.ReloadCommand)
		if err != nil {
			s.log.WithErr(err).Warn("执行远程重载命令失败")
			reloadOutput = fmt.Sprintf("执行失败: %v", err)
		} else {
			reloadOutput = output
		}
	}

	// 9. 返回结果
	result := map[string]interface{}{
		"host":           config.Host,
		"fullchain_path": fullchainPath,
		"privkey_path":   privkeyPath,
		"reload_output":  reloadOutput,
	}
	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// deployToAliyunCAS 部署到阿里云证书服务
func (s *DeployService) deployToAliyunCAS(ctx context.Context, target *model.DeployTarget, fullchainPem, privkeyPem, domainsJSON string) (string, error) {
	// 1. 解析运行时配置（已由 App 层填充完整）
	var config model.AliyunCASDeployRuntimeConfig
	if err := json.Unmarshal([]byte(target.Config), &config); err != nil {
		return "", s.err.New("解析阿里云 CAS 部署配置失败", err)
	}

	// 注意：配置已由 App 层解密，无需再次解密

	// 2. 解析域名（单域名字符串）
	domain := strings.TrimSpace(domainsJSON)
	if domain == "" {
		return "", s.err.New("域名不能为空", nil).ValidWithCtx()
	}
	domains := []string{domain}

	// 3. 创建 OpenAPI 配置（用于多个服务客户端）
	openAPIConfig := &openapi.Config{
		AccessKeyId:     tea.String(config.AccessKeyID),
		AccessKeySecret: tea.String(config.AccessKeySecret),
	}

	// 设置地域 endpoint
	region := config.Region
	if region == "" {
		region = "cn-hangzhou" // 默认杭州
	}

	// 4. 上传证书到 CAS
	openAPIConfig.Endpoint = tea.String(fmt.Sprintf("cas.%s.aliyuncs.com", region))
	casClient, err := cas.NewClient(openAPIConfig)
	if err != nil {
		return "", s.err.New("创建阿里云 CAS 客户端失败", err)
	}

	// 生成唯一证书名（避免重名）
	certName := s.generateUniqueCertName(domains[0])

	uploadReq := &cas.UploadUserCertificateRequest{
		Name: tea.String(certName),
		Cert: tea.String(fullchainPem),
		Key:  tea.String(privkeyPem),
	}

	uploadResp, err := casClient.UploadUserCertificate(uploadReq)
	if err != nil {
		return "", s.err.New("上传证书到阿里云 CAS 失败", err)
	}

	certId := tea.Int64Value(uploadResp.Body.CertId)
	s.log.WithFields(map[string]interface{}{
		"cert_name": certName,
		"cert_id":   certId,
	}).Info("证书上传到阿里云 CAS 成功")

	result := map[string]interface{}{
		"cert_id":        certId,
		"cert_name":      certName,
		"region":         region,
		"domains":        domains,
		"deployed_to":    []string{},
		"deploy_results": []map[string]interface{}{},
	}

	// 5. 如果启用了自动部署，则部署到云产品服务
	if config.AutoDeploy {
		s.log.WithField("domains", domains).Info("开始自动部署证书到云产品服务")

		deployResults := []map[string]interface{}{}
		deployedServices := []string{}

		// 部署到 CDN
		cdnResults := s.deployToCDN(openAPIConfig, domains, certName, fullchainPem, privkeyPem)
		if len(cdnResults) > 0 {
			deployResults = append(deployResults, cdnResults...)
			deployedServices = append(deployedServices, "CDN")
		}

		// 部署到 DCDN
		dcdnResults := s.deployToDCDN(openAPIConfig, domains, certName, fullchainPem, privkeyPem)
		if len(dcdnResults) > 0 {
			deployResults = append(deployResults, dcdnResults...)
			deployedServices = append(deployedServices, "DCDN")
		}

		// 部署到 OSS
		ossResults := s.deployToOSS(config.AccessKeyID, config.AccessKeySecret, region, domains, fullchainPem, privkeyPem)
		if len(ossResults) > 0 {
			deployResults = append(deployResults, ossResults...)
			deployedServices = append(deployedServices, "OSS")
		}

		result["deployed_to"] = deployedServices
		result["deploy_results"] = deployResults

		s.log.WithFields(map[string]interface{}{
			"deployed_services": deployedServices,
			"total_resources":   len(deployResults),
		}).Info("自动部署完成")
	}

	// 6. 返回结果
	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// generateUniqueCertName 生成唯一证书名称
// 格式: <sanitized_domain>-<unix_nano>
func (s *DeployService) generateUniqueCertName(domain string) string {
	// 清洗域名：只保留字母、数字、短横线
	var sanitized strings.Builder
	for _, ch := range domain {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' {
			sanitized.WriteRune(ch)
		} else if ch == '.' || ch == '*' {
			sanitized.WriteRune('-')
		}
	}

	// 生成时间戳后缀
	timestamp := time.Now().UnixNano()

	// 组合并限制长度（阿里云证书名称限制）
	certName := fmt.Sprintf("%s-%d", sanitized.String(), timestamp)
	if len(certName) > 100 {
		certName = certName[:100]
	}

	return certName
}

// createSSHClient 创建 SSH 客户端
func (s *DeployService) createSSHClient(config *model.SSHDeployRuntimeConfig) (*ssh.Client, error) {
	var authMethods []ssh.AuthMethod

	if config.AuthMethod == "privatekey" && config.PrivateKey != "" {
		signer, err := ssh.ParsePrivateKey([]byte(config.PrivateKey))
		if err != nil {
			return nil, fmt.Errorf("解析 SSH 私钥失败: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	} else if config.AuthMethod == "password" && config.Password != "" {
		authMethods = append(authMethods, ssh.Password(config.Password))
	} else {
		return nil, fmt.Errorf("无效的 SSH 认证配置")
	}

	port := config.Port
	if port == 0 {
		port = 22
	}

	sshConfig := &ssh.ClientConfig{
		User:            config.Username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // 生产环境应验证 host key
	}

	addr := fmt.Sprintf("%s:%d", config.Host, port)
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("连接 SSH 服务器失败: %w", err)
	}

	return client, nil
}

// uploadFile 上传文件到 SFTP
func (s *DeployService) uploadFile(sftpClient *sftp.Client, remotePath string, content []byte, fileModeStr string) error {
	file, err := sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("创建远程文件失败: %w", err)
	}
	defer file.Close()

	if _, err := file.Write(content); err != nil {
		return fmt.Errorf("写入远程文件失败: %w", err)
	}

	// 设置文件权限
	if fileModeStr != "" {
		if mode, err := strconv.ParseUint(fileModeStr, 8, 32); err == nil {
			if err := sftpClient.Chmod(remotePath, os.FileMode(mode)); err != nil {
				s.log.WithErr(err).Warn("设置远程文件权限失败")
			}
		}
	}

	return nil
}

// executeCommand 执行本地命令
func (s *DeployService) executeCommand(command string) (string, error) {
	cmd := exec.Command("sh", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("命令执行失败: %w", err)
	}
	return string(output), nil
}

// executeSSHCommand 执行 SSH 远程命令
func (s *DeployService) executeSSHCommand(client *ssh.Client, command string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("创建 SSH 会话失败: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(command)
	if err != nil {
		return string(output), fmt.Errorf("命令执行失败: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// deployToCDN 部署证书到 CDN
func (s *DeployService) deployToCDN(config *openapi.Config, domains []string, certName, fullchainPem, privkeyPem string) []map[string]interface{} {
	results := []map[string]interface{}{}

	// 创建 CDN 客户端
	config.Endpoint = tea.String("cdn.aliyuncs.com")
	cdnClient, err := cdn.NewClient(config)
	if err != nil {
		s.log.WithErr(err).Error("创建 CDN 客户端失败")
		return results
	}

	// 遍历域名，查询 CDN 配置并部署证书
	for _, domain := range domains {
		s.log.WithField("domain", domain).Info("检查 CDN 域名")

		// 查询域名配置
		describeReq := &cdn.DescribeCdnDomainDetailRequest{
			DomainName: tea.String(domain),
		}
		describeResp, err := cdnClient.DescribeCdnDomainDetail(describeReq)
		if err != nil {
			s.log.WithErr(err).WithField("domain", domain).Warn("查询 CDN 域名失败，可能域名不存在")
			continue
		}

		// 检查是否启用了 HTTPS
		if describeResp.Body.GetDomainDetailModel == nil {
			continue
		}

		s.log.WithField("domain", domain).Info("发现 CDN 域名，开始部署证书")

		// 设置证书
		setCertReq := &cdn.SetCdnDomainSSLCertificateRequest{
			DomainName:  tea.String(domain),
			CertName:    tea.String(certName),
			CertType:    tea.String("upload"),
			SSLProtocol: tea.String("on"),
			SSLPub:      tea.String(fullchainPem),
			SSLPri:      tea.String(privkeyPem),
		}

		_, err = cdnClient.SetCdnDomainSSLCertificate(setCertReq)
		if err != nil {
			s.log.WithErr(err).WithField("domain", domain).Error("CDN 证书部署失败")
			results = append(results, map[string]interface{}{
				"service": "CDN",
				"domain":  domain,
				"success": false,
				"error":   err.Error(),
			})
		} else {
			s.log.WithField("domain", domain).Info("CDN 证书部署成功")
			results = append(results, map[string]interface{}{
				"service":   "CDN",
				"domain":    domain,
				"success":   true,
				"cert_name": certName,
			})
		}
	}

	return results
}

// deployToDCDN 部署证书到 DCDN（全站加速）
func (s *DeployService) deployToDCDN(config *openapi.Config, domains []string, certName, fullchainPem, privkeyPem string) []map[string]interface{} {
	results := []map[string]interface{}{}

	// 创建 DCDN 客户端
	config.Endpoint = tea.String("dcdn.aliyuncs.com")
	dcdnClient, err := dcdn.NewClient(config)
	if err != nil {
		s.log.WithErr(err).Error("创建 DCDN 客户端失败")
		return results
	}

	// 遍历域名，查询 DCDN 配置并部署证书
	for _, domain := range domains {
		s.log.WithField("domain", domain).Info("检查 DCDN 域名")

		// 查询域名配置
		describeReq := &dcdn.DescribeDcdnDomainDetailRequest{
			DomainName: tea.String(domain),
		}
		describeResp, err := dcdnClient.DescribeDcdnDomainDetail(describeReq)
		if err != nil {
			s.log.WithErr(err).WithField("domain", domain).Warn("查询 DCDN 域名失败，可能域名不存在")
			continue
		}

		// 检查域名是否存在
		if describeResp.Body.DomainDetail == nil {
			continue
		}

		s.log.WithField("domain", domain).Info("发现 DCDN 域名，开始部署证书")

		// 设置证书
		setCertReq := &dcdn.SetDcdnDomainSSLCertificateRequest{
			DomainName:  tea.String(domain),
			CertName:    tea.String(certName),
			CertType:    tea.String("upload"),
			SSLProtocol: tea.String("on"),
			SSLPub:      tea.String(fullchainPem),
			SSLPri:      tea.String(privkeyPem),
		}

		_, err = dcdnClient.SetDcdnDomainSSLCertificate(setCertReq)
		if err != nil {
			s.log.WithErr(err).WithField("domain", domain).Error("DCDN 证书部署失败")
			results = append(results, map[string]interface{}{
				"service": "DCDN",
				"domain":  domain,
				"success": false,
				"error":   err.Error(),
			})
		} else {
			s.log.WithField("domain", domain).Info("DCDN 证书部署成功")
			results = append(results, map[string]interface{}{
				"service":   "DCDN",
				"domain":    domain,
				"success":   true,
				"cert_name": certName,
			})
		}
	}

	return results
}

// deployToOSS 部署证书到 OSS（对象存储）
func (s *DeployService) deployToOSS(accessKeyID, accessKeySecret, region string, domains []string, fullchainPem, privkeyPem string) []map[string]interface{} {
	results := []map[string]interface{}{}

	// 构建 OSS endpoint
	endpoint := fmt.Sprintf("oss-%s.aliyuncs.com", region)
	if region == "" {
		endpoint = "oss-cn-hangzhou.aliyuncs.com"
	}

	// 创建 OSS 客户端
	client, err := oss.New(endpoint, accessKeyID, accessKeySecret)
	if err != nil {
		s.log.WithErr(err).Error("创建 OSS 客户端失败")
		return results
	}

	// 列举所有 Bucket
	marker := ""
	for {
		listResult, err := client.ListBuckets(oss.Marker(marker), oss.MaxKeys(100))
		if err != nil {
			s.log.WithErr(err).Error("列举 OSS Bucket 失败")
			break
		}

		// 遍历每个 Bucket，检查自定义域名
		for _, bucket := range listResult.Buckets {
			s.log.WithField("bucket", bucket.Name).Debug("检查 OSS Bucket")

			// 获取 Bucket 的自定义域名列表（返回 XML 字符串）
			cnameXML, err := client.GetBucketCname(bucket.Name)
			if err != nil {
				s.log.WithErr(err).WithField("bucket", bucket.Name).Debug("获取 Bucket CNAME 失败")
				continue
			}

			// 解析 XML
			var cnameResult GetBucketCnameResult
			if err := xml.Unmarshal([]byte(cnameXML), &cnameResult); err != nil {
				s.log.WithErr(err).WithField("bucket", bucket.Name).Warn("解析 CNAME XML 失败")
				continue
			}

			// 检查自定义域名是否匹配证书域名
			for _, cnameInfo := range cnameResult.Cnames {
				if s.domainMatchesCert(cnameInfo.Domain, domains) {
					s.log.WithFields(map[string]interface{}{
						"bucket": bucket.Name,
						"domain": cnameInfo.Domain,
					}).Info("发现匹配的 OSS 自定义域名，开始部署证书")

					// 部署证书
					err := s.deployOSSCertificate(client, bucket.Name, cnameInfo.Domain, fullchainPem, privkeyPem)
					if err != nil {
						s.log.WithErr(err).WithFields(map[string]interface{}{
							"bucket": bucket.Name,
							"domain": cnameInfo.Domain,
						}).Error("OSS 证书部署失败")
						results = append(results, map[string]interface{}{
							"service": "OSS",
							"bucket":  bucket.Name,
							"domain":  cnameInfo.Domain,
							"success": false,
							"error":   err.Error(),
						})
					} else {
						s.log.WithFields(map[string]interface{}{
							"bucket": bucket.Name,
							"domain": cnameInfo.Domain,
						}).Info("OSS 证书部署成功")
						results = append(results, map[string]interface{}{
							"service": "OSS",
							"bucket":  bucket.Name,
							"domain":  cnameInfo.Domain,
							"success": true,
						})
					}
				}
			}
		}

		// 检查是否还有更多 Bucket
		if !listResult.IsTruncated {
			break
		}
		marker = listResult.NextMarker
	}

	return results
}

// deployOSSCertificate 为 OSS Bucket 的自定义域名部署证书
func (s *DeployService) deployOSSCertificate(client *oss.Client, bucketName, domain, certPem, keyPem string) error {
	// 构建 CNAME 和证书配置
	putCnameConfig := oss.PutBucketCname{
		Cname: domain,
		CertificateConfiguration: &oss.CertificateConfiguration{
			CertId:      "", // 使用证书内容时可以为空
			Certificate: certPem,
			PrivateKey:  keyPem,
			Force:       true, // 强制更新
		},
	}

	// 为域名配置证书
	err := client.PutBucketCnameWithCertificate(bucketName, putCnameConfig)
	if err != nil {
		return fmt.Errorf("配置 OSS 证书失败: %w", err)
	}

	return nil
}

// domainMatchesCert 检查域名是否匹配证书域名列表（支持通配符）
func (s *DeployService) domainMatchesCert(domain string, certDomains []string) bool {
	for _, certDomain := range certDomains {
		if certDomain == domain {
			return true
		}

		// 处理通配符域名，如 *.example.com
		if strings.HasPrefix(certDomain, "*.") {
			wildcardBase := certDomain[2:] // 去掉 "*."
			if strings.HasSuffix(domain, "."+wildcardBase) || domain == wildcardBase {
				return true
			}
		}
	}
	return false
}
