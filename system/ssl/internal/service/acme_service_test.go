package service

import (
	"testing"

	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/system/ssl/internal/model"
)

func newTestAcmeService() *AcmeService {
	return NewAcmeService(logger.GetLogger())
}

func TestCreateDNSProvider_GoDaddy(t *testing.T) {
	svc := newTestAcmeService()
	_, err := svc.createDNSProvider(model.DnsProviderGoDaddy, "test-api-key", "test-api-secret")
	if err != nil {
		t.Fatalf("createDNSProvider(godaddy) 期望成功，得到错误: %v", err)
	}
}

func TestCreateDNSProvider_AliDNS(t *testing.T) {
	svc := newTestAcmeService()
	_, err := svc.createDNSProvider(model.DnsProviderAliDNS, "test-access-key", "test-secret-key")
	if err != nil {
		t.Fatalf("createDNSProvider(alidns) 期望成功，得到错误: %v", err)
	}
}

func TestCreateDNSProvider_TencentCloud(t *testing.T) {
	svc := newTestAcmeService()
	_, err := svc.createDNSProvider(model.DnsProviderTencentCloud, "test-secret-id", "test-secret-key")
	if err != nil {
		t.Fatalf("createDNSProvider(tencentcloud) 期望成功，得到错误: %v", err)
	}
}

func TestCreateDNSProvider_DNSPod_MapsToTencentCloud(t *testing.T) {
	svc := newTestAcmeService()
	_, err := svc.createDNSProvider(model.DnsProviderDNSPod, "test-secret-id", "test-secret-key")
	if err != nil {
		t.Fatalf("createDNSProvider(dnspod) 期望成功，得到错误: %v", err)
	}
}

func TestCreateDNSProvider_Unknown(t *testing.T) {
	svc := newTestAcmeService()
	_, err := svc.createDNSProvider("unknown-provider", "key", "secret")
	if err == nil {
		t.Fatal("createDNSProvider(unknown) 期望返回错误，但没有")
	}
}
