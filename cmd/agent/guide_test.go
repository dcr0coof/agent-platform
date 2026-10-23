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

func TestCLIGuideRoundTrip(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "agent.exe")
	if out, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	path := filepath.Join(dir, "guide.md")
	if err := os.WriteFile(path, []byte("# 自制示例\n\n杭州雨天先核对场馆预约。"), 0600); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []llm.Message `json:"messages"`
			Tools    []llm.ToolDef `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if calls.Add(1) == 1 {
			found := false
			for _, def := range body.Tools {
				if def.Function.Name == "search_travel_guide" {
					found = true
				}
			}
			if !found {
				t.Error("guide tool not registered")
			}
			w.Write([]byte(`{"choices":[{"message":{"tool_calls":[{"id":"guide-1","type":"function","function":{"name":"search_travel_guide","arguments":"{\"query\":\"杭州 雨天\"}"}}]},"finish_reason":"tool_calls"}]}`))
			return
		}
		last := body.Messages[len(body.Messages)-1]
		if last.Role != llm.RoleTool || last.ToolCallID != "guide-1" || !strings.Contains(last.Content, `"start_line":3`) || !strings.Contains(last.Content, "杭州雨天先核对场馆预约。") || !strings.Contains(last.Content, `"sha256":`) {
			t.Errorf("missing evidence: %+v", last)
		}
		w.Write([]byte(`{"choices":[{"message":{"content":"mock guide evidence received"},"finish_reason":"stop"}]}`))
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "-guide-file", path)
	for _, env := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(env), "AGENT_") {
			cmd.Env = append(cmd.Env, env)
		}
	}
	cmd.Env = append(cmd.Env, "AGENT_LLM_API_KEY=fake-key", "AGENT_LLM_BASE_URL="+server.URL)
	cmd.Stdin = strings.NewReader("杭州雨天攻略\n/exit\n")
	out, err := cmd.CombinedOutput()
	if err != nil || calls.Load() != 2 || !strings.Contains(string(out), "mock guide evidence received") {
		t.Fatalf("round trip: %v calls=%d %s", err, calls.Load(), out)
	}
}
