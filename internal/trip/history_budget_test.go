package trip

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/dcr0coof/agent-platform/internal/llm"
	"github.com/dcr0coof/agent-platform/internal/tool"
)

func TestHistoryByteBudgetPreservesWholeNewestTurns(t *testing.T) {
	call := llm.ToolCall{ID: "weather-1", Type: "function"}
	call.Function.Name, call.Function.Arguments = "weather", `{"city":"杭州"}`
	second := call
	second.ID = "weather-2"
	newest := []llm.Message{
		{Role: llm.RoleUser, Content: "天气 <&>\n"},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{call, second}},
		{Role: llm.RoleTool, ToolCallID: call.ID, Content: "天气未知"},
		{Role: llm.RoleTool, ToolCallID: second.ID, Content: "预报未知"},
		{Role: llm.RoleAssistant, Content: "请稍后重试"},
	}
	older := []llm.Message{{Role: llm.RoleUser, Content: "以前"}, {Role: llm.RoleAssistant, Content: "答复"}}
	history := append(append([]llm.Message{}, older...), newest...)
	encoded, _ := json.Marshal(newest)
	all, _ := json.Marshal(history)
	for _, tc := range []struct {
		name  string
		limit int
		want  []llm.Message
	}{
		{"exact whole tool turn", len(encoded), newest},
		{"one byte short drops entire newest", len(encoded) - 1, history[len(history):]},
		{"both turns exact", len(all), history},
		{"one byte short drops oldest", len(all) - 1, newest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, size, err := historyWithinBytes(history, tc.limit)
			actual, _ := json.Marshal(got)
			if err != nil || !reflect.DeepEqual(got, tc.want) || size != len(actual) || size > tc.limit {
				t.Fatalf("got=%+v bytes=%d actual=%d err=%v", got, size, len(actual), err)
			}
		})
	}
	unchanged, _ := json.Marshal(history)
	if string(all) != string(unchanged) {
		t.Fatal("input history changed")
	}
}

func TestHistoryByteBudgetKeepsContiguousSuffix(t *testing.T) {
	history := []llm.Message{
		{Role: llm.RoleUser, Content: "small old"}, {Role: llm.RoleAssistant, Content: "old"},
		{Role: llm.RoleUser, Content: "large"}, {Role: llm.RoleAssistant, Content: strings.Repeat("x", 1000)},
		{Role: llm.RoleUser, Content: "newest"}, {Role: llm.RoleAssistant, Content: "new"},
	}
	got, _, err := historyWithinBytes(history, 300)
	if err != nil || !reflect.DeepEqual(got, history[4:]) {
		t.Fatalf("non-contiguous context: %+v %v", got, err)
	}
}

func TestHistoryByteBudgetMatchesRequestAndPreservesSavedHistory(t *testing.T) {
	for _, oversizedNewest := range []bool{false, true} {
		t.Run(fmt.Sprint(oversizedNewest), func(t *testing.T) {
			count := 0
			large := strings.Repeat("界", historyByteLimit/3+1)
			runner := AgentRunner{Tools: tool.NewRegistry(), MaxMessages: 20, MaxIterations: 1,
				Client: modelFunc(func(_ context.Context, messages []llm.Message, _ []llm.ToolDef) (*llm.Response, error) {
					count++
					if count == 3 {
						want := 4 // system + latest small turn + input
						if oversizedNewest {
							want = 2
						}
						if len(messages) != want {
							t.Errorf("model messages=%d want=%d", len(messages), want)
						}
						for _, m := range messages {
							if m.Content == large {
								t.Error("large history sent")
							}
						}
					}
					if count == 1 && !oversizedNewest || count == 2 && oversizedNewest {
						return &llm.Response{Content: large}, nil
					}
					return &llm.Response{Content: "小答复"}, nil
				}),
			}
			h := setup(t, runner)
			ss := h.newSession()
			var run Run
			for i := 0; i < 3; i++ {
				h.request("POST", "/api/sessions/"+ss.ID+"/runs", startBody("继续", fmt.Sprintf("history-bytes-%d", i), ss.Revision), 202, &run)
				run = h.wait(run.ID)
				if run.Status != "completed" {
					t.Fatalf("run failed: %+v", run)
				}
				h.request("GET", "/api/sessions/"+ss.ID, nil, 200, &ss)
			}
			if len(ss.Messages) != 6 || !strings.Contains(encode(ss.Messages), large) {
				t.Fatal("saved history was trimmed")
			}
			want := "载入 2/4 条消息、1/2 个完整回合"
			if oversizedNewest {
				want = "载入 0/4 条消息、0/2 个完整回合"
			}
			found := false
			for _, event := range run.Events {
				if event.Type == "context.ready" && strings.Contains(event.Detail, want) && strings.Contains(event.Detail, "/131072 字节") {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing budget event: %+v", run.Events)
			}
		})
	}
}
