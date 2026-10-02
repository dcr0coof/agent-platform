package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRequestBudgetCountsExactSerializedBytes(t *testing.T) {
	var calls, received atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		body, _ := io.ReadAll(r.Body)
		received.Store(int64(len(body)))
		fmt.Fprint(w, `{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}]}`)
	}))
	defer server.Close()
	client := NewOpenAI("test", server.URL, "test", 20, 0)
	base, err := json.Marshal(openaiChatRequest{Model: "test", Messages: []Message{{Role: RoleUser}}, MaxTokens: 20})
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range []int{MaxRequestBytes - 1, MaxRequestBytes, MaxRequestBytes + 1} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			before := calls.Load()
			_, err := client.Chat(context.Background(), []Message{{Role: RoleUser, Content: strings.Repeat("x", size-len(base))}}, nil)
			if size <= MaxRequestBytes {
				if err != nil || calls.Load() != before+1 || received.Load() != int64(size) {
					t.Fatalf("size=%d received=%d err=%v", size, received.Load(), err)
				}
			} else if err == nil || calls.Load() != before || !strings.Contains(err.Error(), "1048577 字节") {
				t.Fatalf("oversized request was not rejected before transport: calls=%d err=%v", calls.Load(), err)
			}
		})
	}
}

func TestRequestBudgetIncludesUTF8EscapingAndToolMetadata(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(500) }))
	defer server.Close()
	client := NewOpenAI("never-echo-this-key", server.URL, "test", 20, 0)
	toolDef := ToolDef{Type: "function"}
	toolDef.Function.Name = "evidence"
	toolDef.Function.Parameters = map[string]interface{}{"description": strings.Repeat("x", MaxRequestBytes)}
	call := ToolCall{ID: "one", Type: "function"}
	call.Function.Name, call.Function.Arguments = "evidence", strings.Repeat("x", MaxRequestBytes)
	for _, tc := range []struct {
		name     string
		messages []Message
		tools    []ToolDef
	}{
		{"utf8", []Message{{Role: RoleUser, Content: strings.Repeat("旅", MaxRequestBytes/3+1)}}, nil},
		{"json escaping", []Message{{Role: RoleUser, Content: strings.Repeat("\n", MaxRequestBytes/2)}}, nil},
		{"system", []Message{{Role: RoleSystem, Content: strings.Repeat("x", MaxRequestBytes)}}, nil},
		{"tools", []Message{{Role: RoleUser, Content: "hi"}}, []ToolDef{toolDef}},
		{"arguments", []Message{{Role: RoleAssistant, ToolCalls: []ToolCall{call}}}, nil},
		{"tool result", []Message{{Role: RoleTool, ToolCallID: "one", Content: strings.Repeat("x", MaxRequestBytes)}}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := client.Chat(context.Background(), tc.messages, tc.tools)
			if err == nil || !strings.Contains(err.Error(), "本次未发送") || strings.Contains(err.Error(), "never-echo-this-key") || calls.Load() != 0 {
				t.Fatalf("budget/transport/privacy: calls=%d err=%v", calls.Load(), err)
			}
		})
	}
}
