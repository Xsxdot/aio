package utils

import (
	"fmt"
	"math/rand"
	"time"
)

// GenerateOutTradeNo 生成商户订单号
// 格式：时间戳(14位) + 随机数(6位)
func GenerateOutTradeNo() string {
	timestamp := time.Now().Format("20060102150405")
	random := rand.Intn(999999-100000) + 100000
	return fmt.Sprintf("%s%d", timestamp, random)
}

// GenerateOutRefundNo 生成商户退款单号
// 格式：R + 时间戳(14位) + 随机数(6位)
func GenerateOutRefundNo() string {
	timestamp := time.Now().Format("20060102150405")
	random := rand.Intn(999999-100000) + 100000
	return fmt.Sprintf("R%s%d", timestamp, random)
}

// GenerateOrderNo 生成订单号
// 格式：ORDER + 日期(8位) + 随机数(8位)
func GenerateOrderNo() string {
	date := time.Now().Format("20060102")
	random := rand.Intn(99999999-10000000) + 10000000
	return fmt.Sprintf("ORDER%s%d", date, random)
}
