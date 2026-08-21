// 本文件定义工作流模块的进程配置。
//
// 职责：承载回调投递方式等 workflow 配置。
// 边界：只描述配置结构，不执行回调或改变调度行为。
package config

// WorkflowConfig 工作流模块配置。
type WorkflowConfig struct {
	// CallbackMode 任务完成回调的投递方式。
	//
	// 取值：
	//   - ""（默认）/ "outbox"：AckJob 事务内提交 outbox 任务，由 aio 内部 worker 消费，失败可重试
	//   - "sync"：退回改造前的同步进程内回调，失败即丢失
	//
	// 刻意用字符串而非 bool：struct 反序列化时 bool 零值为 false，
	// 若定义成「默认 true 的 bool」，配置缺失反而会得到 false。
	// 字符串空值语义明确——未配置即新行为，退回必须显式声明。
	//
	// 这是过渡开关，稳定运行一个版本后应连同 sync 分支一并删除。
	CallbackMode string `yaml:"callback-mode" json:"callback-mode"`
}
