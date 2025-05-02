package authmanager_test

import (
	"fmt"

	"github.com/xsxdot/aio/pkg/auth"

	"github.com/xsxdot/aio/internal/authmanager"
	"github.com/xsxdot/aio/internal/etcd"
)

// Example_basic 展示基本用法示例
func Example_basic() {
	// 本示例仅作演示，不会真正运行

	// 创建etcd客户端
	// etcdClient, _ := etcd.NewEtcdClient(etcdConfig, nil)

	// 仅示例：假设我们已有etcdClient
	var etcdClient *etcd.EtcdClient

	// 创建存储提供者
	storage := authmanager.NewEtcdStorage(etcdClient)

	// 创建认证管理器
	manager, err := authmanager.NewAuthManager(storage, nil)
	if err != nil {
		fmt.Printf("创建认证管理器失败: %v\n", err)
		return
	}

	// 创建普通用户
	normalUser := &authmanager.User{
		Username:    "user1",
		DisplayName: "Normal User",
		Phone:       "12345678901",
		Status:      "active",
	}
	err = manager.CreateUser(normalUser, "password123", []string{"reader"})
	if err != nil {
		fmt.Printf("创建用户失败: %v\n", err)
		return
	}

	// 创建角色
	readerRole := &auth.Role{
		ID:          "reader",
		Name:        "Reader",
		Description: "Can read resources but not modify",
		Permissions: []auth.Permission{
			{Resource: "contents", Action: "read"},
			{Resource: "dashboard", Action: "view"},
		},
	}
	err = manager.SaveRole(readerRole)
	if err != nil {
		fmt.Printf("创建角色失败: %v\n", err)
		return
	}

	// 用户登录
	loginReq := authmanager.LoginRequest{
		Username: "user1",
		Password: "password123",
	}
	loginResp, err := manager.Login(loginReq, "127.0.0.1", "Mozilla/5.0")
	if err != nil {
		fmt.Printf("用户登录失败: %v\n", err)
		return
	}

	// 验证权限
	token := loginResp.Token.AccessToken

	// 有权限的操作
	resp, err := manager.VerifyPermission(token, "contents", "read")
	if err != nil {
		fmt.Printf("权限验证失败: %v\n", err)
		return
	}
	if resp.Allowed {
		fmt.Println("用户有权读取内容")
	} else {
		fmt.Printf("权限被拒绝: %s\n", resp.Reason)
	}

	// 无权限的操作
	resp, err = manager.VerifyPermission(token, "contents", "write")
	if err != nil {
		fmt.Printf("权限验证失败: %v\n", err)
		return
	}
	if !resp.Allowed {
		fmt.Println("用户无权写入内容")
	}

	// 更新用户密码
	err = manager.UpdateUserPassword(normalUser.ID, "password123", "newpassword456")
	if err != nil {
		fmt.Printf("更新密码失败: %v\n", err)
		return
	}
	fmt.Println("密码更新成功")

	// 获取所有用户
	users, err := manager.ListUsers()
	if err != nil {
		fmt.Printf("获取用户列表失败: %v\n", err)
		return
	}
	fmt.Printf("系统中共有 %d 个用户\n", len(users))

	// 输出:
	// 用户有权读取内容
	// 用户无权写入内容
	// 密码更新成功
	// 系统中共有 2 个用户
}
