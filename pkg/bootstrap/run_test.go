// run_test.go 验证 AIO bootstrap 启动流程中的诊断信息。
//
// 职责：
//   - 覆盖注册中心自注册失败时的日志消息构造
//   - 保证启动 panic 不吞掉 SDK 返回的底层错误
//
// 边界：
//   - 不连接真实注册中心
//   - 不启动 bootstrap 组件生命周期
package bootstrap

import (
	"errors"
	"strings"
	"testing"

	"github.com/xsxdot/aio/pkg/sdk"
)

func TestRegisterSelfFailureMessageIncludesRootCauseAndContext(t *testing.T) {
	bootCfg := LocalBootstrap{
		Env: "dev",
		Aio: SdkConfig{RegistryAddr: "100.84.251.46:50051"},
	}
	svcReq := &sdk.EnsureServiceRequest{
		Project: "aihub",
		Name:    "aihub",
	}
	instReq := &sdk.RegisterInstanceRequest{
		InstanceKey: "aihub-0.0.0.0-1780000000",
		Endpoint:    "0.0.0.0:10100",
		Env:         "dev",
	}
	cause := errors.New("ensure service failed: rpc error: code = Unavailable desc = connection reset by peer")

	msg := registerSelfFailureMessage(bootCfg, svcReq, instReq, cause)

	for _, want := range []string{
		"failed to register self to registry",
		"registry_addr=100.84.251.46:50051",
		"project=aihub",
		"service_name=aihub",
		"env=dev",
		"instance_key=aihub-0.0.0.0-1780000000",
		"endpoint=0.0.0.0:10100",
		cause.Error(),
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("failure message %q does not contain %q", msg, want)
		}
	}
}
