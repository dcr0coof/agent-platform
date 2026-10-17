package trip

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/dcr0coof/agent-platform/internal/llm"
	"github.com/dcr0coof/agent-platform/internal/tool"
)

func TestModelKnowsOnlyActualHistoryOmissions(t *testing.T) {
	old := []llm.Message{{Role: llm.RoleUser, Content: "omitted-secret-preference"}, {Role: llm.RoleAssistant, Content: "old answer"}}
	latest := []llm.Message{{Role: llm.RoleUser, Content: "recent"}, {Role: llm.RoleAssistant, Content: "recent answer"}}
	large := []llm.Message{{Role: llm.RoleUser, Content: "omitted-secret-preference"}, {Role: llm.RoleAssistant, Content: strings.Repeat("x", historyByteLimit)}}
	for _, tc := range []struct {
		name          string
		max           int
		history, want []llm.Message
		notice        string
	}{
		{"new conversation", 20, nil, []llm.Message{}, ""},
		{"full history", 20, latest, latest, ""},
		{"message window", 2, append(append([]llm.Message{}, old...), latest...), latest, "载入 2/4 条历史消息、1/2 个完整回合"},
		{"byte window", 20, append(append([]llm.Message{}, large...), latest...), latest, "载入 2/4 条历史消息、1/2 个完整回合"},
		{"empty retained history", 20, large, []llm.Message{}, "载入 0/2 条历史消息、0/1 个完整回合"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := encode(tc.history)
			calls := 0
			runner := AgentRunner{Tools: tool.NewRegistry(), MaxMessages: tc.max, MaxIterations: 2,
				Client: modelFunc(func(_ context.Context, messages []llm.Message, _ []llm.ToolDef) (*llm.Response, error) {
					calls++
					prompt := messages[0].Content
					if (tc.notice != "") != strings.Contains(prompt, "运行时历史边界") {
						t.Fatalf("incorrect omission claim: %s", prompt)
					}
					if tc.notice != "" && (!strings.Contains(prompt, tc.notice) || !strings.Contains(prompt, "请用户补充") || strings.Contains(prompt, "omitted-secret-preference")) {
						t.Fatal("missing boundary or omitted content leaked")
					}
					if !strings.Contains(prompt, "不去博物馆") || messages[0].Role != llm.RoleSystem {
						t.Fatal("confirmed constraints missing")
					}
					if !reflect.DeepEqual(messages[1:1+len(tc.want)], tc.want) || messages[1+len(tc.want)].Content != "按之前说的继续" {
						t.Fatal("history or input changed")
					}
					if calls == 1 {
						call := llm.ToolCall{ID: "probe", Type: "function"}
						call.Function.Name, call.Function.Arguments = "missing_tool", "{}"
						return &llm.Response{ToolCalls: []llm.ToolCall{call}}, nil
					}
					if messages[len(messages)-2].ToolCalls[0].ID != "probe" || messages[len(messages)-1].ToolCallID != "probe" {
						t.Fatal("tool pair broken")
					}
					return &llm.Response{Content: "请补充之前的条件"}, nil
				}),
			}
			added, err := runner.Execute(context.Background(), Session{Messages: tc.history, Constraints: Constraints{Preferences: "不去博物馆"}}, "按之前说的继续", func(string, string) error { return nil })
			if err != nil || calls != 2 || len(added) != 4 || encode(tc.history) != before {
				t.Fatalf("history/run changed: calls=%d added=%d err=%v", calls, len(added), err)
			}
			for _, m := range added {
				if m.Role == llm.RoleSystem || strings.Contains(m.Content, "运行时历史边界") {
					t.Fatal("runtime notice persisted into conversation")
				}
			}
		})
	}
}
