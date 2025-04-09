package server

import (
	"fmt"
	"github.com/xsxdot/aio/internal/cache/protocol"
	"testing"
)

// TestRoleCommandOutput 测试ROLE命令输出格式
func TestRoleCommandOutput(t *testing.T) {
	// 手动构建从节点角色的响应，与execRole方法生成的一致
	slaveRoleValues := []string{"slave", "127.0.0.1", "7379", "connected", "0"}
	slaveReply := protocol.NewMultiBulkReply(slaveRoleValues)

	fmt.Printf("ROLE slave response: %s\n", slaveReply.String())
	fmt.Printf("ROLE slave bytes: %s\n", string(slaveReply.ToMessage().ToBytes()))

	// 手动构建主节点角色的响应
	masterRoleValues := []string{"master", "0"}
	masterReply := protocol.NewMultiBulkReply(masterRoleValues)

	fmt.Printf("ROLE master response: %s\n", masterReply.String())
	fmt.Printf("ROLE master bytes: %s\n", string(masterReply.ToMessage().ToBytes()))

	// 手动构建未知角色的响应
	noneRoleValues := []string{"none"}
	noneReply := protocol.NewMultiBulkReply(noneRoleValues)

	fmt.Printf("ROLE none response: %s\n", noneReply.String())
	fmt.Printf("ROLE none bytes: %s\n", string(noneReply.ToMessage().ToBytes()))
}
