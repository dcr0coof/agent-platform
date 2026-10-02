package trip

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/dcr0coof/agent-platform/internal/llm"
	"github.com/dcr0coof/agent-platform/internal/tool"
)

type oversizedEvidence struct{}

func (oversizedEvidence) Name() string { return "oversized_evidence" }
func (oversizedEvidence) Description() string {
	return "Synthetic evidence for request budget verification"
}
func (oversizedEvidence) Parameters() map[string]interface{} {
	return map[string]interface{}{"type": "object"}
}
func (oversizedEvidence) Execute(context.Context, map[string]interface{}) (string, error) {
	return strings.Repeat("旅", llm.MaxRequestBytes/3+1), nil
}

func TestRequestBudgetRejectsToolGrowthWithoutCommittingTurn(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		fmt.Fprint(w, `{"choices":[{"message":{"tool_calls":[{"id":"evidence-one","type":"function","function":{"name":"oversized_evidence","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`)
	}))
	defer server.Close()
	registry := tool.NewRegistry()
	registry.Register(oversizedEvidence{})
	runner := AgentRunner{Client: llm.NewOpenAI("test", server.URL, "test", 20, 0), Tools: registry, MaxMessages: 20, MaxIterations: 3}
	ss := Session{Messages: []llm.Message{{Role: llm.RoleUser, Content: "之前的问题"}, {Role: llm.RoleAssistant, Content: "之前的回答"}}}
	before := encode(ss.Messages)
	var completed int
	added, err := runner.Execute(context.Background(), ss, "检索资料", func(kind, detail string) error {
		if kind == "tool.completed" {
			completed++
		}
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "本次未发送") || calls.Load() != 1 || completed != 1 || len(added) != 0 || encode(ss.Messages) != before {
		t.Fatalf("calls=%d completed=%d added=%d err=%v", calls.Load(), completed, len(added), err)
	}
}
