package memory

import (
	"github.com/dcr0coof/agent-platform/internal/llm"
	"testing"
)

func TestBufferPreservesWholeToolTurn(t *testing.T) {
	m := NewBuffer(3)
	m.Add(llm.Message{Role: llm.RoleUser, Content: "old"})
	m.Add(llm.Message{Role: llm.RoleAssistant, Content: "old reply"})
	m.Add(llm.Message{Role: llm.RoleUser, Content: "calculate"})
	m.Add(llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "a"}, {ID: "b"}}})
	m.Add(llm.Message{Role: llm.RoleTool, ToolCallID: "a", Content: "1"})
	m.Add(llm.Message{Role: llm.RoleTool, ToolCallID: "b", Content: "2"})
	m.Add(llm.Message{Role: llm.RoleAssistant, Content: "done"})
	all := m.All()
	if len(all) != 5 || all[0].Role != llm.RoleUser || all[1].Role != llm.RoleAssistant {
		t.Fatalf("tool turn was split: %+v", all)
	}
	m.Add(llm.Message{Role: llm.RoleUser, Content: "next"})
	if m.Len() != 1 {
		t.Fatalf("old oversized turn retained: %d", m.Len())
	}
}

func TestBufferOwnsToolCalls(t *testing.T) {
	m := NewBuffer(3)
	msg := llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "original"}}}
	m.Add(msg)
	msg.ToolCalls[0].ID = "mutated input"
	all := m.All()
	all[0].ToolCalls[0].ID = "mutated output"
	if m.All()[0].ToolCalls[0].ID != "original" {
		t.Fatal("history aliases caller memory")
	}
}
