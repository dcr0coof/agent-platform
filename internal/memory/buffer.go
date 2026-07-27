package memory

import "github.com/dcr0coof/agent-platform/internal/llm"

// BufferMemory 滑动窗口记忆
// 保留最近 maxMessages 条消息 + system prompt
type BufferMemory struct {
	messages    []llm.Message
	maxMessages int
	system      string
}

// NewBuffer 创建缓冲区记忆
func NewBuffer(maxMessages int) *BufferMemory {
	return &BufferMemory{
		maxMessages: maxMessages,
		messages:    make([]llm.Message, 0, maxMessages),
	}
}

func (m *BufferMemory) Add(msg llm.Message) {
	m.messages = append(m.messages, msg)
	// 超出窗口，丢掉最旧的非 system 消息
	if len(m.messages) > m.maxMessages {
		m.messages = m.messages[len(m.messages)-m.maxMessages:]
	}
}

func (m *BufferMemory) All() []llm.Message {
	result := make([]llm.Message, 0, len(m.messages)+1)
	if m.system != "" {
		result = append(result, llm.Message{Role: llm.RoleSystem, Content: m.system})
	}
	result = append(result, m.messages...)
	return result
}

func (m *BufferMemory) SetSystem(prompt string) {
	m.system = prompt
}

func (m *BufferMemory) Clear() {
	m.messages = make([]llm.Message, 0, m.maxMessages)
}

func (m *BufferMemory) Len() int {
	return len(m.messages)
}
