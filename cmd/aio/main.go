// Package main 提供AIO服务的入口点
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/xsxdot/aio/app"
)

// 版本信息
var (
	Version   = "1.0.0"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func main() {
	// 解析命令行参数
	configPath := flag.String("config", "./conf", "配置文件路径")
	showVersion := flag.Bool("version", false, "显示版本信息")
	flag.Parse()

	// 显示版本信息
	if *showVersion {
		fmt.Printf("AIO 服务 v%s\n", Version)
		fmt.Printf("构建时间: %s\n", BuildTime)
		fmt.Printf("Git 提交: %s\n", GitCommit)
		return
	}

	// 打印启动信息
	log.Printf("启动 AIO 服务 v%s...\n", Version)
	log.Printf("使用配置文件: %s\n", *configPath)

	// 创建应用实例
	application := app.New()

	// 加载配置
	if err := application.LoadConfig(*configPath); err != nil {
		log.Fatalf("加载配置失败: %v\n", err)
	}

	// 启动应用
	if err := application.Start(); err != nil {
		log.Fatalf("启动应用失败: %v\n", err)
	}

	// 等待信号
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	// 收到信号后优雅退出
	sig := <-signalChan
	log.Printf("收到信号 %v，正在优雅退出...\n", sig)

	// 停止应用
	if err := application.Stop(); err != nil {
		log.Printf("停止应用失败: %v\n", err)
	}

	log.Println("AIO 服务已停止")
}
