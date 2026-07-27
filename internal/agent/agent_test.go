package agent

import (
	"context"
	"testing"

	"github.com/dcr0coof/agent-platform/internal/llm"
	"github.com/dcr0coof/agent-platform/internal/memory"
	"github.com/dcr0coof/agent-platform/internal/tool"
	"github.com/dcr0coof/agent-platform/internal/tool/builtin"
)

// mockLLM 模拟 LLM 客户端
type mockLLM struct {
	responses []*llm.Response
	callCount int
}

func (m *mockLLM) Chat(ctx context.Context, messages []llm.Message, tools []llm.ToolDef) (*llm.Response, error) {
	if m.callCount >= len(m.responses) {
		return &llm.Response{Content: "done"}, nil
	}
	resp := m.responses[m.callCount]
	m.callCount++
	return resp, nil
}

func TestAgent_SimpleTextResponse(t *testing.T) {
	mock := &mockLLM{
		responses: []*llm.Response{
			{Content: "你好！有什么可以帮你的？"},
		},
	}

	reg := tool.NewRegistry()
	mem := memory.NewBuffer(10)
	agent := New("test", mock, mem, reg, 5)

	result, err := agent.Run(context.Background(), "你好")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result != "你好！有什么可以帮你的？" {
		t.Errorf("unexpected: %s", result)
	}
}

func TestAgent_ToolCallLoop(t *testing.T) {
	// 模拟 LLM 先返回工具调用，再返回文本
	mock := &mockLLM{
		responses: []*llm.Response{
			{
				ToolCalls: []llm.ToolCall{
					{
						ID:   "call_1",
						Type: "function",
						Function: struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						}{Name: "calculator", Arguments: `{"expression":"2+3"}`},
					},
				},
			},
			{Content: "2+3 = 5"},
		},
	}

	reg := tool.NewRegistry()
	reg.Register(&builtin.Calculator{})
	mem := memory.NewBuffer(10)
	agent := New("test", mock, mem, reg, 5)

	result, err := agent.Run(context.Background(), "2+3等于几")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result != "2+3 = 5" {
		t.Errorf("unexpected: %s", result)
	}
}
