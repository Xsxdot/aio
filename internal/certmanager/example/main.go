package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"aio/internal/certmanager"
)

func main() {
	// 命令行参数
	var (
		email           = flag.String("email", "", "Let's Encrypt 账户邮箱")
		domain          = flag.String("domain", "", "要申请证书的域名")
		certDir         = flag.String("cert-dir", "./certs", "证书存储目录")
		staging         = flag.Bool("staging", true, "使用Let's Encrypt测试环境")
		enabled         = flag.Bool("enabled", true, "启用证书管理")
		verifyMethod    = flag.String("verify", "http-01", "验证方式: http-01 或 dns-01")
		dnsProvider     = flag.String("dns-provider", "", "DNS提供商: aliyun")
		aliyunKeyID     = flag.String("aliyun-key-id", "", "阿里云AccessKey ID")
		aliyunKeySecret = flag.String("aliyun-key-secret", "", "阿里云AccessKey Secret")
		aliyunRegion    = flag.String("aliyun-region", "cn-hangzhou", "阿里云区域")
	)
	flag.Parse()

	// 验证必须参数
	if *email == "" || *domain == "" {
		fmt.Println("必须提供邮箱和域名")
		fmt.Println("使用方法: certmanager -email your@email.com -domain yourdomain.com [options]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// 处理验证方式
	var verifyMethodEnum certmanager.VerifyMethod
	if *verifyMethod == "dns-01" {
		verifyMethodEnum = certmanager.VerifyDNS

		// 验证DNS参数
		if *dnsProvider == "" {
			fmt.Println("使用DNS验证方式必须指定DNS提供商")
			flag.PrintDefaults()
			os.Exit(1)
		}

		// 验证阿里云参数
		if *dnsProvider == "aliyun" && (*aliyunKeyID == "" || *aliyunKeySecret == "") {
			fmt.Println("使用阿里云DNS验证必须提供AccessKey ID和Secret")
			flag.PrintDefaults()
			os.Exit(1)
		}
	} else {
		verifyMethodEnum = certmanager.VerifyHTTP
	}

	// 设置日志
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	// 创建DNS配置
	dnsConfig := certmanager.DNSConfig{}
	if verifyMethodEnum == certmanager.VerifyDNS && *dnsProvider == "aliyun" {
		dnsConfig.AliyunAccessKeyID = *aliyunKeyID
		dnsConfig.AliyunAccessKeySecret = *aliyunKeySecret
		dnsConfig.AliyunRegionID = *aliyunRegion
	}

	// 创建配置
	config := &certmanager.Config{
		Enabled:               *enabled,
		CertDir:               *certDir,
		Email:                 *email,
		Domains:               []string{*domain},
		RenewBefore:           30,
		Staging:               *staging,
		CheckInterval:         24 * time.Hour,
		VerifyMethod:          verifyMethodEnum,
		DNSProvider:           certmanager.DNSProviderType(*dnsProvider),
		DNSConfig:             dnsConfig,
		DNSPropagationTimeout: 120 * time.Second,
	}

	// 创建服务
	service := certmanager.NewService(config, logger)

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动服务
	if err := service.Start(ctx); err != nil {
		logger.Fatalf("启动证书管理服务失败: %v", err)
	}

	logger.Infof("证书管理服务已启动，监听域名: %s", *domain)

	// 手动申请或更新证书
	cert, err := service.GetCertificate(*domain)
	if err != nil {
		logger.Errorf("获取证书失败: %v", err)
	} else {
		logger.Infof("成功获取证书: %s (过期时间: %s)", cert.Domain, cert.ExpiryDate)
		logger.Infof("证书文件路径: %s", cert.CertFile)
		logger.Infof("私钥文件路径: %s", cert.KeyFile)
		if cert.IsWildcard {
			logger.Infof("这是一个通配符证书，适用于 %s 的所有子域名", cert.Domain[1:]) // 去掉*
		}
	}

	// 等待中断信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	// 停止服务
	logger.Info("正在停止证书管理服务...")
	if err := service.Stop(); err != nil {
		logger.Errorf("停止服务时出错: %v", err)
	}
	logger.Info("证书管理服务已停止")
}
