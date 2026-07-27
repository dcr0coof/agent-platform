package memory

import (
	"testing"

	"github.com/dcr0coof/agent-platform/internal/llm"
)

func TestBufferMemory_Add(t *testing.T) {
	m := NewBuffer(5)

	m.Add(llm.Message{Role: llm.RoleUser, Content: "hello"})
	m.Add(llm.Message{Role: llm.RoleAssistant, Content: "hi"})

	if m.Len() != 2 {
		t.Errorf("expected 2, got %d", m.Len())
	}
}

func TestBufferMemory_CircularOverflow(t *testing.T) {
	m := NewBuffer(3)

	for i := 0; i < 5; i++ {
		m.Add(llm.Message{Role: llm.RoleUser, Content: "msg"})
	}

	// 窗口最多 3 条
	if m.Len() > 3 {
		t.Errorf("overflow not trimmed: %d messages", m.Len())
	}
}

func TestBufferMemory_SystemPrompt(t *testing.T) {
	m := NewBuffer(10)
	m.SetSystem("你是一个有用的助手")

	m.Add(llm.Message{Role: llm.RoleUser, Content: "你好"})

	all := m.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 messages (system + user), got %d", len(all))
	}
	if all[0].Role != llm.RoleSystem {
		t.Errorf("first message should be system, got %s", all[0].Role)
	}
	if all[0].Content != "你是一个有用的助手" {
		t.Errorf("wrong system prompt: %s", all[0].Content)
	}
}

func TestBufferMemory_Clear(t *testing.T) {
	m := NewBuffer(10)
	m.SetSystem("system")
	m.Add(llm.Message{Role: llm.RoleUser, Content: "hello"})

	m.Clear()

	if m.Len() != 0 {
		t.Errorf("expected 0 after clear, got %d", m.Len())
	}

	// system prompt preserved
	all := m.All()
	if len(all) != 1 || all[0].Role != llm.RoleSystem {
		t.Fatal("system prompt should survive clear")
	}
}
