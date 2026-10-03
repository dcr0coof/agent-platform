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

func TestRequestBudgetFailureFeedbackAndRecovery(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 2 {
			fmt.Fprint(w, `{"choices":[{"message":{"tool_calls":[{"id":"oversized","type":"function","function":{"name":"oversized_evidence","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`)
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"content":"正常回复"},"finish_reason":"stop"}]}`)
	}))
	defer server.Close()
	registry := tool.NewRegistry()
	registry.Register(oversizedEvidence{})
	h := setup(t, AgentRunner{Client: llm.NewOpenAI("private-test-key", server.URL, "test", 20, 0), Tools: registry, MaxMessages: 20, MaxIterations: 3})
	s := h.newSession()
	start := func(key string) Run {
		t.Helper()
		var r Run
		h.request("POST", "/api/sessions/"+s.ID+"/runs", startBody(key, "budget-test-"+key, s.Revision), 202, &r)
		return h.wait(r.ID)
	}
	if r := start("before"); r.Status != "completed" {
		t.Fatal(r)
	}
	h.request("GET", "/api/sessions/"+s.ID, nil, 200, &s)
	before := encode(s.Messages)
	failed := start("oversized")
	if failed.Status != "failed" || !strings.Contains(failed.Error, "1048576 字节上限") || !strings.Contains(failed.Error, "开启新会话") || !strings.Contains(failed.Error, "此前模型请求或工具调用可能已执行") || strings.Contains(failed.Error, "private-test-key") {
		t.Fatalf("unexpected feedback: %+v", failed)
	}
	if last := failed.Events[len(failed.Events)-1]; last.Type != "run.failed" || last.Detail != failed.Error {
		t.Fatalf("SSE replay feedback differs: %+v", last)
	}
	h.request("GET", "/api/sessions/"+s.ID, nil, 200, &s)
	if encode(s.Messages) != before {
		t.Fatal("failed turn changed history")
	}
	if r := start("recovery"); r.Status != "completed" {
		t.Fatal(r)
	}
	h.request("GET", "/api/sessions/"+s.ID, nil, 200, &s)
	if len(s.Messages) != 4 || calls.Load() != 3 {
		t.Fatalf("messages=%d calls=%d", len(s.Messages), calls.Load())
	}
}

func TestRequestBudgetFeedbackDoesNotExposeWrapper(t *testing.T) {
	h := setup(t, runnerFunc(func(context.Context, Session, string, func(string, string) error) ([]llm.Message, error) {
		return nil, fmt.Errorf("private wrapper: %w", &llm.RequestSizeError{Bytes: llm.MaxRequestBytes + 1})
	}))
	s := h.newSession()
	var r Run
	h.request("POST", "/api/sessions/"+s.ID+"/runs", startBody("test", "budget-test-wrapped", s.Revision), 202, &r)
	r = h.wait(r.ID)
	if !strings.Contains(r.Error, "1048577") || strings.Contains(encode(r), "private wrapper") {
		t.Fatalf("unsafe or missing feedback: %+v", r)
	}
}
