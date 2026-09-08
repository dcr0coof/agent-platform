package memory

import (
	"github.com/dcr0coof/agent-platform/internal/llm"
	"sync"
)

// BufferMemory 滑动窗口记忆
// 按完整用户回合裁剪；最近一个回合可以超过 maxMessages，避免切断工具调用链。
type BufferMemory struct {
	mu          sync.RWMutex
	messages    []llm.Message
	maxMessages int
	system      string
}

// NewBuffer 创建缓冲区记忆
func NewBuffer(maxMessages int) *BufferMemory {
	if maxMessages <= 0 {
		maxMessages = 20
	}
	return &BufferMemory{
		maxMessages: maxMessages,
		messages:    make([]llm.Message, 0, maxMessages),
	}
}

func (m *BufferMemory) Add(msg llm.Message) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, cloneMessage(msg))
	for len(m.messages) > m.maxMessages {
		next := 0
		for i := 1; i < len(m.messages); i++ {
			if m.messages[i].Role == llm.RoleUser {
				next = i
				break
			}
		}
		if next == 0 {
			break
		}
		// 复制并清零尾部，释放旧消息引用，复用已有容量。
		n := copy(m.messages, m.messages[next:])
		clear(m.messages[n:])
		m.messages = m.messages[:n]
	}
}

func (m *BufferMemory) All() []llm.Message {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]llm.Message, 0, len(m.messages)+1)
	if m.system != "" {
		result = append(result, llm.Message{Role: llm.RoleSystem, Content: m.system})
	}
	for _, msg := range m.messages {
		result = append(result, cloneMessage(msg))
	}
	return result
}

func (m *BufferMemory) SetSystem(prompt string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.system = prompt
}

func (m *BufferMemory) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = make([]llm.Message, 0, m.maxMessages)
}

func (m *BufferMemory) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.messages)
}

func cloneMessage(msg llm.Message) llm.Message {
	if msg.ToolCalls != nil {
		msg.ToolCalls = append([]llm.ToolCall(nil), msg.ToolCalls...)
	}
	return msg
}
