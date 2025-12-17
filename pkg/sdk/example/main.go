package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"xiaozhizhang/pkg/sdk"
)

func main() {
	// 从环境变量读取配置
	registryAddr := os.Getenv("REGISTRY_ADDR")
	if registryAddr == "" {
		registryAddr = "localhost:50051"
	}

	clientKey := os.Getenv("CLIENT_KEY")
	if clientKey == "" {
		fmt.Println("CLIENT_KEY is required")
		os.Exit(1)
	}

	clientSecret := os.Getenv("CLIENT_SECRET")
	if clientSecret == "" {
		fmt.Println("CLIENT_SECRET is required")
		os.Exit(1)
	}

	// 创建 SDK 客户端
	fmt.Printf("Creating SDK client (registry: %s)...\n", registryAddr)
	client, err := sdk.New(sdk.Config{
		RegistryAddr: registryAddr,
		ClientKey:    clientKey,
		ClientSecret: clientSecret,
	})
	if err != nil {
		fmt.Printf("Failed to create SDK client: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	fmt.Println("SDK client created successfully")

	// 1. 测试认证 - 获取 token
	fmt.Println("\n=== Step 1: Authentication ===")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	token, err := client.Auth.Token(ctx)
	cancel()
	if err != nil {
		fmt.Printf("Failed to get token: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Token obtained: %s...\n", token[:20])

	// 2. 拉取服务列表
	fmt.Println("\n=== Step 2: List Services ===")
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	services, err := client.Registry.ListServices(ctx, "aio", "dev")
	cancel()
	if err != nil {
		fmt.Printf("Failed to list services: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found %d services:\n", len(services))
	for _, svc := range services {
		fmt.Printf("  - %s/%s: %d instances\n", svc.Project, svc.Name, len(svc.Instances))
		for _, inst := range svc.Instances {
			fmt.Printf("    * %s (%s)\n", inst.Endpoint, inst.Env)
		}
	}

	// 3. 测试服务发现（如果有服务）
	if len(services) > 0 {
		fmt.Println("\n=== Step 3: Service Discovery ===")
		targetService := services[0]

		// 尝试 Pick 3 次，测试 round-robin
		for i := 0; i < 3; i++ {
			instance, reportErr, err := client.Discovery.Pick(
				targetService.Project,
				targetService.Name,
				"dev",
			)
			if err != nil {
				fmt.Printf("Failed to pick instance: %v\n", err)
				break
			}
			fmt.Printf("  Round %d: picked %s\n", i+1, instance.Endpoint)

			// 模拟成功调用（不报告错误）
			reportErr(nil)
		}
	}

	// 4. 测试配置中心客户端（可选）
	fmt.Println("\n=== Step 4: Config Client (Optional) ===")
	{
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// 尝试获取配置（如果配置中心有数据）
		configJSON, err := client.ConfigClient.GetConfigJSON(ctx, "test.config", "dev")
		if err != nil {
			fmt.Printf("  Get config failed (expected if no config exists): %v\n", err)
		} else {
			fmt.Printf("  Config retrieved: %s\n", configJSON)
		}
	}

	// 5. 测试短网址客户端（可选）
	fmt.Println("\n=== Step 5: ShortURL Client (Optional) ===")
	{
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// 尝试解析短链接（演示 API 调用，实际可能失败）
		_, err := client.ShortURL.Resolve(ctx, "short.example.com", "test", "")
		if err != nil {
			fmt.Printf("  Resolve short link failed (expected if link doesn't exist): %v\n", err)
		} else {
			fmt.Println("  Short link resolved successfully")
		}
	}

	// 6. 注册自身到注册中心
	fmt.Println("\n=== Step 6: Register Self ===")

	// 首先需要有一个 service（这里假设已经在注册中心创建了 test-sdk 服务）
	// 实际使用时需要先通过管理接口创建 service
	var testServiceID int64
	if len(services) > 0 {
		testServiceID = services[0].ID
	} else {
		fmt.Println("No service found, skipping self registration")
		goto cleanup
	}

	{
		hostname, _ := os.Hostname()
		if hostname == "" {
			hostname = "sdk-example"
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		handle, err := client.Registry.RegisterSelf(ctx, &sdk.RegisterInstanceRequest{
			ServiceID:   testServiceID,
			InstanceKey: fmt.Sprintf("sdk-example-%d", time.Now().Unix()),
			Env:         "dev",
			Host:        hostname,
			Endpoint:    fmt.Sprintf("http://%s:8080", hostname),
			MetaJSON:    `{"sdk":"go","version":"0.1.0"}`,
			TTLSeconds:  60,
		})
		cancel()

		if err != nil {
			fmt.Printf("Failed to register self: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Successfully registered as: %s\n", handle.InstanceKey)
		fmt.Println("Heartbeat is running in background...")

		// 7. 运行一段时间，等待信号退出
		fmt.Println("\n=== Step 7: Running (Press Ctrl+C to stop) ===")
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				fmt.Printf("[%s] Still running, heartbeat active...\n", time.Now().Format("15:04:05"))
			case sig := <-sigCh:
				fmt.Printf("\nReceived signal: %v\n", sig)
				fmt.Println("Deregistering...")

				err := handle.Stop()
				if err != nil {
					fmt.Printf("Failed to stop registration: %v\n", err)
				} else {
					fmt.Println("Deregistered successfully")
				}

				goto cleanup
			}
		}
	}

cleanup:
	fmt.Println("\n=== Cleanup ===")
	fmt.Println("Example completed")
}
