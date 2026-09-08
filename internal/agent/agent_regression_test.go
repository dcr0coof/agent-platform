package agent

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/dcr0coof/agent-platform/internal/llm"
	"github.com/dcr0coof/agent-platform/internal/memory"
	"github.com/dcr0coof/agent-platform/internal/tool"
	"github.com/dcr0coof/agent-platform/internal/tool/builtin"
)

type chatFunc func(context.Context, []llm.Message, []llm.ToolDef) (*llm.Response, error)

func (f chatFunc) Chat(ctx context.Context, m []llm.Message, defs []llm.ToolDef) (*llm.Response, error) {
	return f(ctx, m, defs)
}

func call(id, name, args string) llm.ToolCall {
	tc := llm.ToolCall{ID: id, Type: "function"}
	tc.Function.Name, tc.Function.Arguments = name, args
	return tc
}

func TestAgentFailedTurnDoesNotChangeHistory(t *testing.T) {
	for _, mode := range []string{"llm_error", "nil_response", "empty_response", "iteration_limit", "cancel", "invalid_call"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			mem := memory.NewBuffer(2)
			mem.SetSystem("system")
			mem.Add(llm.Message{Role: llm.RoleUser, Content: "old"})
			mem.Add(llm.Message{Role: llm.RoleAssistant, Content: "old reply"})
			before := mem.All()
			calls := 0
			client := chatFunc(func(context.Context, []llm.Message, []llm.ToolDef) (*llm.Response, error) {
				calls++
				if mode == "iteration_limit" || calls == 1 {
					return &llm.Response{ToolCalls: []llm.ToolCall{call("a", "calculator", `{"expression":"2+3"}`)}}, nil
				}
				switch mode {
				case "llm_error":
					return nil, errors.New("offline")
				case "nil_response":
					return nil, nil
				case "empty_response":
					return &llm.Response{}, nil
				case "cancel":
					cancel()
					return &llm.Response{Content: "late"}, nil
				default:
					return &llm.Response{ToolCalls: []llm.ToolCall{call("", "calculator", `{}`)}}, nil
				}
			})
			reg := tool.NewRegistry()
			reg.Register(&builtin.Calculator{})
			a := New("test", client, mem, reg, 2)
			if _, err := a.Run(ctx, "new"); err == nil {
				t.Fatal("expected failed turn")
			}
			if !reflect.DeepEqual(before, mem.All()) {
				t.Fatalf("failed turn changed history: %+v", mem.All())
			}
		})
	}
}

func TestAgentToolErrorsRemainUsableContext(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Register(&builtin.Calculator{})
	reg.Register(panicTool{})
	mem := memory.NewBuffer(2)
	calls := 0
	client := chatFunc(func(ctx context.Context, messages []llm.Message, defs []llm.ToolDef) (*llm.Response, error) {
		calls++
		if calls == 1 {
			return &llm.Response{Content: "checking", ToolCalls: []llm.ToolCall{
				call("a", "missing", `{}`), call("b", "calculator", `{`),
				call("c", "calculator", `null`), call("d", "panic", `{}`),
			}}, nil
		}
		if len(messages) != 6 || messages[0].Role != llm.RoleUser || messages[1].Content != "checking" {
			t.Fatalf("bad tool history: %+v", messages)
		}
		for i, msg := range messages[2:] {
			if msg.ToolCallID != string(rune('a'+i)) || !strings.Contains(msg.Content, "工具执行失败") {
				t.Errorf("missing tool result: %+v", msg)
			}
		}
		return &llm.Response{Content: "tools failed"}, nil
	})
	a := New("test", client, mem, reg, 2)
	if _, err := a.Run(context.Background(), "try tools"); err != nil {
		t.Fatal(err)
	}
	if mem.Len() != 7 {
		t.Fatalf("successful tool turn truncated: %d", mem.Len())
	}
}

type panicTool struct{}

func (panicTool) Name() string                                                    { return "panic" }
func (panicTool) Description() string                                             { return "panic test" }
func (panicTool) Parameters() map[string]interface{}                              { return nil }
func (panicTool) Execute(context.Context, map[string]interface{}) (string, error) { panic("test") }

func TestAgentConcurrentTurnsAreSerialized(t *testing.T) {
	mem := memory.NewBuffer(100)
	client := chatFunc(func(ctx context.Context, messages []llm.Message, defs []llm.ToolDef) (*llm.Response, error) {
		if len(messages)%2 != 1 {
			t.Errorf("interleaved turns: %+v", messages)
		}
		return &llm.Response{Content: "ok"}, nil
	})
	a := New("test", client, mem, tool.NewRegistry(), 1)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := a.Run(context.Background(), "hello"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if mem.Len() != 40 {
		t.Fatalf("lost messages: %d", mem.Len())
	}
}

func TestAgentCancelledWaitReturns(t *testing.T) {
	a := New("test", &mockLLM{}, memory.NewBuffer(10), tool.NewRegistry(), 1)
	a.runGate <- struct{}{}
	defer func() { <-a.runGate }()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := a.Run(ctx, "hello"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation: %v", err)
	}
}
