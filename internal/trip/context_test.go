package trip

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/dcr0coof/agent-platform/internal/llm"
	"github.com/dcr0coof/agent-platform/internal/tool"
)

func TestContextEvidenceMatchesActualModelRequest(t *testing.T) {
	call := llm.ToolCall{ID: "weather-previous", Type: "function"}
	call.Function.Name = "weather"
	call.Function.Arguments = `{"city":"杭州"}`
	history := []llm.Message{
		{Role: llm.RoleUser, Content: "旧问题"}, {Role: llm.RoleAssistant, Content: "旧回答"},
		{Role: llm.RoleUser, Content: "天气"}, {Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{call}},
		{Role: llm.RoleTool, ToolCallID: call.ID, Content: "未知"}, {Role: llm.RoleAssistant, Content: "天气未知"},
	}
	for _, tc := range []struct {
		name          string
		max           int
		history, want []llm.Message
		detail        string
	}{
		{"oversized newest turn", 2, history, history[2:], "载入 4/6 条消息、1/2 个完整回合；2 条历史消息未载入"},
		{"all turns", 6, history, history, "载入 6/6 条消息、2/2 个完整回合；0 条历史消息未载入"},
		{"default window", 0, history, history, "载入 6/6 条消息、2/2 个完整回合；0 条历史消息未载入"},
		{"first turn", 2, nil, []llm.Message{}, "载入 0/0 条消息、0/0 个完整回合；0 条历史消息未载入"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := encode(tc.history)
			called := 0
			model := modelFunc(func(_ context.Context, messages []llm.Message, _ []llm.ToolDef) (*llm.Response, error) {
				called++
				if len(messages) != len(tc.want)+2 || messages[0].Role != llm.RoleSystem || !strings.Contains(messages[0].Content, "不去博物馆") || messages[len(messages)-1].Content != "新问题" {
					t.Fatalf("wrong system/input context: %+v", messages)
				}
				if !reflect.DeepEqual(messages[1:len(messages)-1], tc.want) {
					t.Fatalf("history differs: %+v", messages)
				}
				return &llm.Response{Content: "新回答"}, nil
			})
			runner := AgentRunner{Client: model, Tools: tool.NewRegistry(), MaxMessages: tc.max, MaxIterations: 1}
			var detail string
			added, err := runner.Execute(context.Background(), Session{Messages: tc.history, Constraints: Constraints{Preferences: "不去博物馆"}}, "新问题", func(kind, value string) error {
				if kind == "context.ready" {
					detail = value
				}
				return nil
			})
			if err != nil || called != 1 || len(added) != 2 || !strings.Contains(detail, tc.detail) || !strings.Contains(detail, "本轮输入与系统约束另计") || !strings.Contains(detail, "非 token 预算") || encode(tc.history) != before {
				t.Fatalf("context evidence/preservation: added=%+v detail=%q called=%d err=%v", added, detail, called, err)
			}
		})
	}
}

func TestContextEventFailurePreventsModelCall(t *testing.T) {
	want := errors.New("event store unavailable")
	runner := AgentRunner{Client: modelFunc(func(context.Context, []llm.Message, []llm.ToolDef) (*llm.Response, error) {
		t.Fatal("model called after event failure")
		return nil, nil
	}), Tools: tool.NewRegistry(), MaxMessages: 2, MaxIterations: 1}
	added, err := runner.Execute(context.Background(), Session{}, "test", func(string, string) error { return want })
	if !errors.Is(err, want) || len(added) != 0 {
		t.Fatalf("got %v %v", added, err)
	}
}

func TestContextEvidenceIsPersistedWithoutTrimmingSavedHistory(t *testing.T) {
	runner := AgentRunner{Client: modelFunc(func(context.Context, []llm.Message, []llm.ToolDef) (*llm.Response, error) {
		return &llm.Response{Content: "完成"}, nil
	}), Tools: tool.NewRegistry(), MaxMessages: 2, MaxIterations: 1}
	h := setup(t, runner)
	ss := h.newSession()
	var run Run
	for _, key := range []string{"context-one", "context-two", "context-three"} {
		h.request("POST", "/api/sessions/"+ss.ID+"/runs", startBody("继续", key, ss.Revision), 202, &run)
		run = h.wait(run.ID)
		if run.Status != "completed" {
			t.Fatalf("run failed: %+v", run)
		}
		h.request("GET", "/api/sessions/"+ss.ID, nil, 200, &ss)
	}
	if len(ss.Messages) != 6 {
		t.Fatalf("stored history was trimmed: %+v", ss.Messages)
	}
	found := false
	for _, event := range run.Events {
		if event.Type == "context.ready" {
			found = strings.Contains(event.Detail, "载入 2/4 条消息、1/2 个完整回合；2 条历史消息未载入")
		}
	}
	if !found {
		t.Fatalf("persisted context evidence missing: %+v", run.Events)
	}
}
