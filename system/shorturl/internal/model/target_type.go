package model

// TargetType 跳转目标类型
type TargetType string

const (
	// TargetTypeURL 普通 URL（直接 302/307）
	TargetTypeURL TargetType = "URL"
	// TargetTypeURLScheme URL Scheme（如 weixin://, alipays:// 等）
	TargetTypeURLScheme TargetType = "URL_SCHEME"
)

// IsValid 检查目标类型是否有效
func (t TargetType) IsValid() bool {
	switch t {
	case TargetTypeURL, TargetTypeURLScheme:
		return true
	}
	return false
}

// NeedsLandingPage 是否需要落地页
func (t TargetType) NeedsLandingPage() bool {
	return t != TargetTypeURL
}


