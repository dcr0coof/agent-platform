package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dcr0coof/agent-platform/internal/llm"
)

// 编译真实 CLI，通过本地 HTTP 服务验证输入 → LLM → 工具 → 清空会话全链路。
func TestCLI(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "agent.exe")
	build := exec.Command("go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []llm.Message `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if r.Header.Get("Authorization") != "Bearer fake-key" {
			t.Error("missing authorization")
		}
		switch requests.Add(1) {
		case 1:
			w.Write([]byte(`{"choices":[{"message":{"tool_calls":[{"id":"a","type":"function","function":{"name":"calculator","arguments":"{\"expression\":\"2+3\"}"}}]},"finish_reason":"tool_calls"}]}`))
		case 2:
			if len(body.Messages) != 4 || body.Messages[3].Content != "2+3 = 5" {
				t.Errorf("bad tool round: %+v", body.Messages)
			}
			w.Write([]byte(`{"choices":[{"message":{"content":"answer: 5"},"finish_reason":"stop"}]}`))
		case 3:
			if len(body.Messages) != 2 || body.Messages[0].Role != llm.RoleSystem {
				t.Errorf("clear failed: %+v", body.Messages)
			}
			w.Write([]byte(`{"choices":[{"message":{"content":"fresh session"},"finish_reason":"stop"}]}`))
		default:
			t.Error("unexpected request")
		}
	}))
	defer server.Close()
	for _, tc := range []struct {
		name, input, want string
		args              []string
		fail              bool
	}{
		{name: "conversation", input: "\n2+3\n/clear\nhello\n/exit\n", want: "fresh session"},
		{name: "eof", input: "", want: "已启动"},
		{name: "help", args: []string{"-h"}, want: "timeout"},
		{name: "invalid_timeout", args: []string{"-timeout=0s"}, want: "timeout 必须大于 0", fail: true},
		{name: "long_input", input: strings.Repeat("x", (1<<20)+1), want: "读取输入失败", fail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary, tc.args...)
			for _, env := range os.Environ() {
				if !strings.HasPrefix(strings.ToUpper(env), "AGENT_") {
					cmd.Env = append(cmd.Env, env)
				}
			}
			cmd.Env = append(cmd.Env, "AGENT_LLM_API_KEY=fake-key", "AGENT_LLM_BASE_URL="+server.URL)
			cmd.Stdin = strings.NewReader(tc.input)
			output, err := cmd.CombinedOutput()
			if ctx.Err() != nil || (err != nil) != tc.fail || !strings.Contains(string(output), tc.want) {
				t.Fatalf("CLI: %v\n%s", err, output)
			}
		})
	}
	if requests.Load() != 3 {
		t.Fatalf("expected 3 LLM calls, got %d", requests.Load())
	}
}
