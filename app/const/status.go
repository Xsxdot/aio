package consts

// ComponentStatus 表示组件状态
type ComponentStatus int

const (
	StatusNotInitialized ComponentStatus = iota
	StatusInitialized
	StatusRunning
	StatusStopped
	StatusError
)
