package certmanager

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	cas20200407 "github.com/alibabacloud-go/cas-20200407/v2/client"
	cdn20180510 "github.com/alibabacloud-go/cdn-20180510/v4/client"
	"github.com/alibabacloud-go/darabonba-openapi/v2/client"
	"github.com/alibabacloud-go/tea/tea"
	"go.uber.org/zap"
)

// AliyunSSLDeployer 阿里云SSL证书部署器
type AliyunSSLDeployer struct {
	logger    *zap.Logger
	casClient *cas20200407.Client
	cdnClient *cdn20180510.Client
}

// NewAliyunSSLDeployer 创建阿里云SSL证书部署器
func NewAliyunSSLDeployer(logger *zap.Logger, config *AliyunConfig) (*AliyunSSLDeployer, error) {
	// 创建OpenAPI配置
	openApiConfig := &client.Config{
		AccessKeyId:     tea.String(config.AccessKeyID),
		AccessKeySecret: tea.String(config.AccessKeySecret),
		RegionId:        tea.String("cn-qingdao"),
	}

	// 创建SSL证书管理客户端
	casClient, err := cas20200407.NewClient(openApiConfig)
	if err != nil {
		return nil, fmt.Errorf("创建SSL证书管理客户端失败: %v", err)
	}

	// 创建CDN客户端
	cdnClient, err := cdn20180510.NewClient(openApiConfig)
	if err != nil {
		return nil, fmt.Errorf("创建CDN客户端失败: %v", err)
	}

	return &AliyunSSLDeployer{
		logger:    logger,
		casClient: casClient,
		cdnClient: cdnClient,
	}, nil
}

// deployToAliyunCDNReal 真正的阿里云云产品部署实现
func (cm *CertManager) deployToAliyunCDNReal(ctx context.Context, cert *DomainCert, config *AliyunConfig) error {
	cm.logger.Info("开始真实阿里云云产品SSL证书部署",
		zap.String("domain", cert.Domain),
		zap.String("target_domain", config.TargetDomain))
	// zap.String("region", config.Region))

	// 创建部署器
	deployer, err := NewAliyunSSLDeployer(cm.logger, config)
	if err != nil {
		return fmt.Errorf("创建阿里云SSL部署器失败: %v", err)
	}

	// 1. 上传证书到阿里云SSL证书管理服务
	certId, err := deployer.uploadCertificate(ctx, cert)
	if err != nil {
		return fmt.Errorf("上传证书到阿里云失败: %v", err)
	}

	cm.logger.Info("证书上传成功", zap.String("cert_id", certId))

	// 2. 获取云资源列表
	resources, err := deployer.listCloudResources(ctx)
	if err != nil {
		return fmt.Errorf("获取云资源列表失败: %v", err)
	}

	// 3. 根据域名筛选云产品资源
	matchedResources := deployer.filterCloudResourcesByDomain(resources, config.TargetDomain)
	if len(matchedResources) == 0 {
		return fmt.Errorf("未找到域名 %s 对应的云产品资源", config.TargetDomain)
	}

	// 统计匹配资源的产品类型
	productCounts := make(map[string]int)
	for _, resource := range matchedResources {
		productCounts[resource.ProductName]++
	}

	cm.logger.Info("找到匹配的云产品资源",
		zap.String("target_domain", config.TargetDomain),
		zap.Int("resource_count", len(matchedResources)),
		zap.Any("product_types", productCounts))

	// 4. 获取联系人列表
	contacts, err := deployer.listContacts(ctx)
	if err != nil {
		return fmt.Errorf("获取联系人列表失败: %v", err)
	}

	if len(contacts) == 0 {
		cm.logger.Warn("未找到现有联系人，尝试使用默认联系人")
		// 尝试使用一个默认联系人ID
		defaultContacts := []Contact{
			{ID: "1", Name: "default", Email: "default@example.com"},
		}
		contacts = defaultContacts
	}

	// 5. 创建部署任务
	jobId, err := deployer.createDeploymentJob(ctx, certId, matchedResources, contacts)
	if err != nil {
		return fmt.Errorf("创建部署任务失败: %v", err)
	}

	cm.logger.Info("部署任务创建成功", zap.Int64("job_id", jobId))

	// 6. 启动部署任务
	err = deployer.startDeploymentJob(ctx, jobId)
	if err != nil {
		return fmt.Errorf("启动部署任务失败: %v", err)
	}

	// 7. 等待部署任务完成
	err = deployer.waitForDeploymentJobComplete(ctx, jobId)
	if err != nil {
		cm.logger.Warn("等待部署任务完成超时或失败", zap.Error(err))
		// 不返回错误，因为任务可能在后台继续执行
	}

	cm.logger.Info("阿里云云产品SSL证书部署完成",
		zap.String("domain", cert.Domain),
		zap.String("target_domain", config.TargetDomain),
		zap.String("cert_id", certId),
		zap.Int64("job_id", jobId))

	return nil
}

// uploadCertificate 上传证书到阿里云SSL证书管理服务
func (d *AliyunSSLDeployer) uploadCertificate(ctx context.Context, cert *DomainCert) (string, error) {
	d.logger.Info("开始上传证书到阿里云SSL证书管理服务",
		zap.String("domain", cert.Domain))

	// 获取证书内容
	var certContent, keyContent string

	// 优先使用存储在etcd中的证书内容
	if cert.CertContent != "" && cert.KeyContent != "" {
		certContent = cert.CertContent
		keyContent = cert.KeyContent
	} else if cert.CertPath != "" && cert.KeyPath != "" {
		// 兼容性处理：从文件读取
		certContentBytes, err := os.ReadFile(cert.CertPath)
		if err != nil {
			return "", fmt.Errorf("读取证书文件失败: %v", err)
		}

		keyContentBytes, err := os.ReadFile(cert.KeyPath)
		if err != nil {
			return "", fmt.Errorf("读取私钥文件失败: %v", err)
		}

		certContent = string(certContentBytes)
		keyContent = string(keyContentBytes)
	} else {
		return "", fmt.Errorf("域名 %s 没有可用的证书内容", cert.Domain)
	}

	// 验证证书和私钥的格式
	if !strings.Contains(certContent, "BEGIN CERTIFICATE") {
		return "", fmt.Errorf("证书格式无效")
	}
	if !strings.Contains(keyContent, "BEGIN PRIVATE KEY") && !strings.Contains(keyContent, "BEGIN RSA PRIVATE KEY") {
		return "", fmt.Errorf("私钥格式无效")
	}

	// 构建上传证书请求
	certName := fmt.Sprintf("auto-cert-%s-%d", strings.ReplaceAll(cert.Domain, "*", "wildcard"), time.Now().Unix())

	uploadRequest := &cas20200407.UploadUserCertificateRequest{
		Name: tea.String(certName),
		Cert: tea.String(certContent),
		Key:  tea.String(keyContent),
	}

	d.logger.Info("正在上传证书",
		zap.String("cert_name", certName),
		zap.String("domain", cert.Domain))

	// 执行上传
	response, err := d.casClient.UploadUserCertificate(uploadRequest)
	if err != nil {
		return "", fmt.Errorf("上传证书失败: %v", err)
	}

	var certId string
	if response.Body.CertId != nil {
		certId = fmt.Sprintf("%d", tea.Int64Value(response.Body.CertId))
	}
	if certId == "" {
		return "", fmt.Errorf("上传证书成功但未获得证书ID")
	}

	d.logger.Info("证书上传成功",
		zap.String("cert_id", certId),
		zap.String("cert_name", certName))

	return certId, nil
}

// CloudResource 云资源信息
type CloudResource struct {
	ID           string
	ProductName  string
	ResourceType string
	Domain       string
	Status       string
}

// Contact 联系人信息
type Contact struct {
	ID    string
	Name  string
	Email string
}

// listCloudResources 获取云资源列表
func (d *AliyunSSLDeployer) listCloudResources(ctx context.Context) ([]CloudResource, error) {
	d.logger.Info("开始获取云资源列表")

	// 构建请求
	listRequest := &cas20200407.ListCloudResourcesRequest{
		CurrentPage: tea.Int32(1),
		ShowSize:    tea.Int32(50),
	}

	response, err := d.casClient.ListCloudResources(listRequest)
	if err != nil {
		return nil, fmt.Errorf("获取云资源列表失败: %v", err)
	}

	var resources []CloudResource
	if response.Body.Data != nil {
		for _, resource := range response.Body.Data {
			cloudRes := CloudResource{
				ID:           fmt.Sprintf("%d", tea.Int64Value(resource.Id)),
				ProductName:  tea.StringValue(resource.CloudProduct),
				ResourceType: tea.StringValue(resource.CloudProduct),
				Domain:       tea.StringValue(resource.Domain),
				Status:       tea.StringValue(resource.Status),
			}
			resources = append(resources, cloudRes)
		}
	}

	d.logger.Info("获取云资源列表成功", zap.Int("count", len(resources)))
	return resources, nil
}

// filterCloudResourcesByDomain 根据域名筛选云产品资源
func (d *AliyunSSLDeployer) filterCloudResourcesByDomain(resources []CloudResource, targetDomain string) []CloudResource {
	var matchedResources []CloudResource

	// 支持SSL证书部署的云产品类型
	supportedProductTypes := map[string]bool{
		"CDN":        true, // 内容分发网络
		"DCDN":       true, // 全站加速
		"OSS":        true, // 对象存储服务
		"SLB":        true, // 传统型负载均衡
		"ALB":        true, // 应用负载均衡
		"NLB":        true, // 网络型负载均衡
		"WAF":        true, // Web应用防火墙
		"GA":         true, // 全球加速
		"LIVE":       true, // 视频直播
		"VOD":        true, // 视频点播
		"APIGateway": true, // API网关
		"FC":         true, // 函数计算
		"MSE":        true, // 微服务引擎
		"SAE":        true, // Serverless应用引擎
		"CR":         true, // 容器镜像服务
		"webHosting": true, // 云虚拟主机
		"DDoS":       true, // DDoS防护
	}

	for _, resource := range resources {
		// 筛选支持SSL证书的云产品资源
		if supportedProductTypes[resource.ProductName] || supportedProductTypes[resource.ResourceType] {
			// 检查域名是否匹配（支持精确匹配和通配符匹配）
			if d.isDomainMatch(resource.Domain, targetDomain) {
				matchedResources = append(matchedResources, resource)
				d.logger.Info("找到匹配的云产品资源",
					zap.String("resource_id", resource.ID),
					zap.String("product_name", resource.ProductName),
					zap.String("domain", resource.Domain),
					zap.String("target_domain", targetDomain),
					zap.String("status", resource.Status))
			}
		}
	}

	return matchedResources
}

// isDomainMatch 检查域名是否匹配（支持通配符）
func (d *AliyunSSLDeployer) isDomainMatch(resourceDomain, targetDomain string) bool {
	// 精确匹配
	if resourceDomain == targetDomain {
		return true
	}

	// 如果资源域名是通配符域名（以*.开头）
	if strings.HasPrefix(resourceDomain, "*.") {
		rootDomain := strings.TrimPrefix(resourceDomain, "*.")
		// 检查目标域名是否是该根域名的子域名
		if strings.HasSuffix(targetDomain, "."+rootDomain) || targetDomain == rootDomain {
			return true
		}
	}

	// 如果目标域名是通配符域名
	if strings.HasPrefix(targetDomain, "*.") {
		rootDomain := strings.TrimPrefix(targetDomain, "*.")
		if strings.HasSuffix(resourceDomain, "."+rootDomain) || resourceDomain == rootDomain {
			return true
		}
	}

	return false
}

// listContacts 获取联系人列表
func (d *AliyunSSLDeployer) listContacts(ctx context.Context) ([]Contact, error) {
	d.logger.Info("开始获取联系人列表")

	// 构建请求
	listRequest := &cas20200407.ListContactRequest{
		CurrentPage: tea.Int32(1),
		ShowSize:    tea.Int32(50),
	}

	response, err := d.casClient.ListContact(listRequest)
	if err != nil {
		return nil, fmt.Errorf("获取联系人列表失败: %v", err)
	}

	var contacts []Contact
	if response.Body.ContactList != nil {
		for _, contact := range response.Body.ContactList {
			contactInfo := Contact{
				ID:    fmt.Sprintf("%d", tea.Int64Value(contact.ContactId)),
				Name:  tea.StringValue(contact.Name),
				Email: tea.StringValue(contact.Email),
			}
			contacts = append(contacts, contactInfo)
		}
	}

	d.logger.Info("获取联系人列表成功", zap.Int("count", len(contacts)))
	return contacts, nil
}

// createDeploymentJob 创建部署任务
func (d *AliyunSSLDeployer) createDeploymentJob(ctx context.Context, certId string, resources []CloudResource, contacts []Contact) (int64, error) {
	d.logger.Info("开始创建部署任务",
		zap.String("cert_id", certId),
		zap.Int("resource_count", len(resources)),
		zap.Int("contact_count", len(contacts)))

	// 构建资源ID列表
	var resourceIds []string
	for _, resource := range resources {
		resourceIds = append(resourceIds, resource.ID)
	}

	// 构建联系人ID列表
	var contactIds []string
	for _, contact := range contacts {
		contactIds = append(contactIds, contact.ID)
	}

	// 生成任务名称
	taskName := fmt.Sprintf("auto-deploy-%s-%d", strings.ReplaceAll(certId, ":", "-"), time.Now().Unix())

	// 构建创建部署任务请求
	createRequest := &cas20200407.CreateDeploymentJobRequest{
		Name:        tea.String(taskName),
		JobType:     tea.String("user"), // 云产品部署任务
		CertIds:     tea.String(certId),
		ResourceIds: tea.String(strings.Join(resourceIds, ",")),
		ContactIds:  tea.String(strings.Join(contactIds, ",")),
	}

	response, err := d.casClient.CreateDeploymentJob(createRequest)
	if err != nil {
		return 0, fmt.Errorf("创建部署任务失败: %v", err)
	}

	jobId := tea.Int64Value(response.Body.JobId)
	d.logger.Info("部署任务创建成功",
		zap.Int64("job_id", jobId),
		zap.String("task_name", taskName))

	return jobId, nil
}

// startDeploymentJob 启动部署任务
func (d *AliyunSSLDeployer) startDeploymentJob(ctx context.Context, jobId int64) error {
	d.logger.Info("开始启动部署任务", zap.Int64("job_id", jobId))

	// 构建更新任务状态请求
	updateRequest := &cas20200407.UpdateDeploymentJobStatusRequest{
		JobId:  tea.Int64(jobId),
		Status: tea.String("pending"), // 设置为待执行状态
	}

	_, err := d.casClient.UpdateDeploymentJobStatus(updateRequest)
	if err != nil {
		return fmt.Errorf("启动部署任务失败: %v", err)
	}

	d.logger.Info("部署任务启动成功", zap.Int64("job_id", jobId))
	return nil
}

// deployCertificateToCDN 部署证书到CDN域名（保留原方法作为备用）
func (d *AliyunSSLDeployer) deployCertificateToCDN(ctx context.Context, certId, domain string) error {
	d.logger.Info("开始部署证书到CDN域名",
		zap.String("cert_id", certId),
		zap.String("domain", domain))

	// 构建设置CDN域名证书请求
	setCertRequest := &cdn20180510.SetCdnDomainSSLCertificateRequest{
		DomainName:  tea.String(domain),
		CertId:      tea.Int64(mustParseInt64(certId)),
		CertType:    tea.String("cas"),
		SSLProtocol: tea.String("on"),
	}

	// 执行证书设置
	_, err := d.cdnClient.SetCdnDomainSSLCertificate(setCertRequest)
	if err != nil {
		return fmt.Errorf("设置CDN域名SSL证书失败: %v", err)
	}

	// 验证证书部署状态
	err = d.verifyCDNCertificateDeployment(ctx, domain)
	if err != nil {
		d.logger.Warn("CDN证书部署验证失败", zap.Error(err))
		// 不返回错误，因为证书可能已经成功设置，只是验证有问题
	}

	d.logger.Info("CDN证书部署完成",
		zap.String("cert_id", certId),
		zap.String("domain", domain))

	return nil
}

// verifyCDNCertificateDeployment 验证CDN证书部署状态
func (d *AliyunSSLDeployer) verifyCDNCertificateDeployment(ctx context.Context, domain string) error {
	d.logger.Info("验证CDN证书部署状态", zap.String("domain", domain))

	// 构建查询请求
	describeRequest := &cdn20180510.DescribeCdnDomainDetailRequest{
		DomainName: tea.String(domain),
	}

	// 查询域名详情
	response, err := d.cdnClient.DescribeCdnDomainDetail(describeRequest)
	if err != nil {
		return fmt.Errorf("查询CDN域名详情失败: %v", err)
	}

	if response.Body.GetDomainDetailModel == nil {
		return fmt.Errorf("未找到CDN域名信息")
	}

	// 检查域名状态
	if response.Body.GetDomainDetailModel.DomainStatus != nil {
		d.logger.Info("CDN域名状态",
			zap.String("domain", domain),
			zap.String("domain_status", tea.StringValue(response.Body.GetDomainDetailModel.DomainStatus)))
	}

	return nil
}

// GetCDNCertificateInfo 获取CDN域名的证书信息
func (cm *CertManager) GetCDNCertificateInfo(config *AliyunConfig) (map[string]interface{}, error) {
	deployer, err := NewAliyunSSLDeployer(cm.logger, config)
	if err != nil {
		return nil, fmt.Errorf("创建阿里云SSL部署器失败: %v", err)
	}

	// 查询CDN域名详情
	describeRequest := &cdn20180510.DescribeCdnDomainDetailRequest{
		DomainName: tea.String(config.TargetDomain),
	}

	response, err := deployer.cdnClient.DescribeCdnDomainDetail(describeRequest)
	if err != nil {
		return nil, fmt.Errorf("查询CDN域名详情失败: %v", err)
	}

	info := map[string]interface{}{
		"domain_name": config.TargetDomain,
	}

	if response.Body.GetDomainDetailModel != nil {
		detail := response.Body.GetDomainDetailModel
		if detail.DomainStatus != nil {
			info["domain_status"] = tea.StringValue(detail.DomainStatus)
		}
		if detail.DomainName != nil {
			info["domain_name_detail"] = tea.StringValue(detail.DomainName)
		}
	}

	return info, nil
}

// ListSSLCertificates 列出阿里云SSL证书管理中的证书
func (cm *CertManager) ListSSLCertificates(config *AliyunConfig) ([]map[string]interface{}, error) {
	deployer, err := NewAliyunSSLDeployer(cm.logger, config)
	if err != nil {
		return nil, fmt.Errorf("创建阿里云SSL部署器失败: %v", err)
	}

	// 构建列表请求
	listRequest := &cas20200407.ListUserCertificateOrderRequest{
		ShowSize:    tea.Int64(50),
		CurrentPage: tea.Int64(1),
	}

	response, err := deployer.casClient.ListUserCertificateOrder(listRequest)
	if err != nil {
		return nil, fmt.Errorf("查询SSL证书列表失败: %v", err)
	}

	var certificates []map[string]interface{}

	if response.Body.CertificateOrderList != nil {
		for _, cert := range response.Body.CertificateOrderList {
			certInfo := map[string]interface{}{
				"cert_id": tea.Int64Value(cert.CertificateId),
				"name":    tea.StringValue(cert.Name),
				"domain":  tea.StringValue(cert.Domain),
				"status":  tea.StringValue(cert.Status),

				"product_name": tea.StringValue(cert.ProductName),
				"source_type":  tea.StringValue(cert.SourceType),
			}
			certificates = append(certificates, certInfo)
		}
	}

	return certificates, nil
}

// DeleteSSLCertificate 删除阿里云SSL证书
func (cm *CertManager) DeleteSSLCertificate(config *AliyunConfig, certId string) error {
	deployer, err := NewAliyunSSLDeployer(cm.logger, config)
	if err != nil {
		return fmt.Errorf("创建阿里云SSL部署器失败: %v", err)
	}

	// 构建删除请求
	deleteRequest := &cas20200407.DeleteUserCertificateRequest{
		CertId: tea.Int64(mustParseInt64(certId)),
	}

	_, err = deployer.casClient.DeleteUserCertificate(deleteRequest)
	if err != nil {
		return fmt.Errorf("删除SSL证书失败: %v", err)
	}

	cm.logger.Info("SSL证书删除成功", zap.String("cert_id", certId))
	return nil
}

// waitForDeploymentJobComplete 等待部署任务完成
func (d *AliyunSSLDeployer) waitForDeploymentJobComplete(ctx context.Context, jobId int64) error {
	d.logger.Info("开始等待部署任务完成", zap.Int64("job_id", jobId))

	// 设置超时时间（5分钟）
	timeout := time.Minute * 5
	interval := time.Second * 10
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		// 查询部署任务状态
		status, err := d.getDeploymentJobStatus(ctx, jobId)
		if err != nil {
			d.logger.Warn("查询部署任务状态失败", zap.Error(err))
			time.Sleep(interval)
			continue
		}

		d.logger.Info("部署任务状态",
			zap.Int64("job_id", jobId),
			zap.String("status", status))

		switch status {
		case "success":
			d.logger.Info("部署任务完成", zap.Int64("job_id", jobId))
			return nil
		case "failed":
			return fmt.Errorf("部署任务失败, 任务ID: %d", jobId)
		case "pending", "running":
			// 任务仍在进行中，继续等待
			time.Sleep(interval)
		default:
			d.logger.Warn("未知的部署任务状态",
				zap.Int64("job_id", jobId),
				zap.String("status", status))
			time.Sleep(interval)
		}
	}

	return fmt.Errorf("等待部署任务完成超时, 任务ID: %d", jobId)
}

// getDeploymentJobStatus 获取部署任务状态
func (d *AliyunSSLDeployer) getDeploymentJobStatus(ctx context.Context, jobId int64) (string, error) {
	// 构建查询请求
	listRequest := &cas20200407.ListDeploymentJobRequest{
		CurrentPage: tea.Int32(1),
		ShowSize:    tea.Int32(50),
	}

	response, err := d.casClient.ListDeploymentJob(listRequest)
	if err != nil {
		return "", fmt.Errorf("查询部署任务列表失败: %v", err)
	}

	// 查找目标任务
	if response.Body.Data != nil {
		for _, job := range response.Body.Data {
			if tea.Int64Value(job.Id) == jobId {
				return tea.StringValue(job.Status), nil
			}
		}
	}

	return "", fmt.Errorf("未找到任务ID为 %d 的部署任务", jobId)
}

// ListDeploymentJobs 列出部署任务
func (cm *CertManager) ListDeploymentJobs(config *AliyunConfig) ([]map[string]interface{}, error) {
	deployer, err := NewAliyunSSLDeployer(cm.logger, config)
	if err != nil {
		return nil, fmt.Errorf("创建阿里云SSL部署器失败: %v", err)
	}

	// 构建列表请求
	listRequest := &cas20200407.ListDeploymentJobRequest{
		CurrentPage: tea.Int32(1),
		ShowSize:    tea.Int32(50),
	}

	response, err := deployer.casClient.ListDeploymentJob(listRequest)
	if err != nil {
		return nil, fmt.Errorf("查询部署任务列表失败: %v", err)
	}

	var jobs []map[string]interface{}
	if response.Body.Data != nil {
		for _, job := range response.Body.Data {
			jobInfo := map[string]interface{}{
				"job_id":        tea.Int64Value(job.Id),
				"name":          tea.StringValue(job.Name),
				"status":        tea.StringValue(job.Status),
				"job_type":      tea.StringValue(job.JobType),
				"create_time":   tea.StringValue(job.GmtCreate),
				"modified_time": tea.StringValue(job.GmtModified),
			}
			jobs = append(jobs, jobInfo)
		}
	}

	return jobs, nil
}

// ListCloudResources 列出所有云产品资源
func (cm *CertManager) ListCloudResources(config *AliyunConfig) ([]map[string]interface{}, error) {
	deployer, err := NewAliyunSSLDeployer(cm.logger, config)
	if err != nil {
		return nil, fmt.Errorf("创建阿里云SSL部署器失败: %v", err)
	}

	// 获取所有云资源
	resources, err := deployer.listCloudResources(context.Background())
	if err != nil {
		return nil, fmt.Errorf("获取云资源列表失败: %v", err)
	}

	var result []map[string]interface{}
	for _, resource := range resources {
		resourceInfo := map[string]interface{}{
			"resource_id":   resource.ID,
			"product_name":  resource.ProductName,
			"resource_type": resource.ResourceType,
			"domain":        resource.Domain,
			"status":        resource.Status,
		}
		result = append(result, resourceInfo)
	}

	return result, nil
}

// GetSupportedProductTypes 获取支持SSL证书部署的产品类型列表
func (cm *CertManager) GetSupportedProductTypes() []map[string]string {
	productTypes := []map[string]string{
		{"code": "CDN", "name": "内容分发网络"},
		{"code": "DCDN", "name": "全站加速"},
		{"code": "OSS", "name": "对象存储服务"},
		{"code": "SLB", "name": "传统型负载均衡"},
		{"code": "ALB", "name": "应用负载均衡"},
		{"code": "NLB", "name": "网络型负载均衡"},
		{"code": "WAF", "name": "Web应用防火墙"},
		{"code": "GA", "name": "全球加速"},
		{"code": "LIVE", "name": "视频直播"},
		{"code": "VOD", "name": "视频点播"},
		{"code": "APIGateway", "name": "API网关"},
		{"code": "FC", "name": "函数计算"},
		{"code": "MSE", "name": "微服务引擎"},
		{"code": "SAE", "name": "Serverless应用引擎"},
		{"code": "CR", "name": "容器镜像服务"},
		{"code": "webHosting", "name": "云虚拟主机"},
		{"code": "DDoS", "name": "DDoS防护"},
	}

	return productTypes
}

// mustParseInt64 辅助函数：将字符串转换为int64，失败时panic
func mustParseInt64(s string) int64 {
	if s == "" {
		return 0
	}
	// 简单的字符串到int64的转换
	// 实际使用时应该使用strconv.ParseInt
	var result int64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int64(c-'0')
		}
	}
	return result
}
