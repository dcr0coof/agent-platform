package memory

import "github.com/dcr0coof/agent-platform/internal/llm"

// Memory 对话记忆接口
type Memory interface {
	// Add 追加一条消息
	Add(msg llm.Message)
	// All 返回完整消息列表（system prompt + history）
	All() []llm.Message
	// SetSystem 设置/更新 system prompt
	SetSystem(prompt string)
	// Clear 清空对话历史（保留 system prompt）
	Clear()
	// Len 返回消息条数（不含 system prompt）
	Len() int
}
