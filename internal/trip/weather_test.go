package trip

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dcr0coof/agent-platform/internal/llm"
	"github.com/dcr0coof/agent-platform/internal/tool"
	"github.com/dcr0coof/agent-platform/internal/tool/builtin"
)

func TestTripWeatherDateIsPreservedThroughToolTurn(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/geo/v2/city/lookup" {
			w.Write([]byte(`{"code":"200","location":[{"id":"101210101"}]}`))
			return
		}
		if r.URL.Path != "/v7/weather/3d" {
			t.Errorf("unexpected weather path: %s", r.URL.Path)
		}
		w.Write([]byte(`{"code":"200","daily":[{"fxDate":"2026-09-17","tempMax":"30","tempMin":"23","textDay":"晴"}]}`))
	}))
	defer provider.Close()
	reg := tool.NewRegistry()
	reg.Register(builtin.NewWeather("test-key", provider.URL))
	calls := 0
	model := modelFunc(func(ctx context.Context, messages []llm.Message, defs []llm.ToolDef) (*llm.Response, error) {
		calls++
		if calls == 1 {
			if len(defs) != 1 || !strings.Contains(encode(defs[0].Function.Parameters), `"date"`) {
				t.Error("date parameter was not exposed to model")
			}
			tc := llm.ToolCall{ID: "weather-date", Type: "function"}
			tc.Function.Name = "weather"
			tc.Function.Arguments = `{"city":"杭州","date":"2026-10-01"}`
			return &llm.Response{ToolCalls: []llm.ToolCall{tc}}, nil
		}
		evidence := messages[len(messages)-1]
		if evidence.Role != llm.RoleTool || evidence.ToolCallID != "weather-date" || !strings.Contains(evidence.Content, "天气未知") || !strings.Contains(evidence.Content, "2026-10-01") || strings.Contains(evidence.Content, "晴") {
			t.Errorf("wrong date evidence passed to model: %+v", evidence)
		}
		return &llm.Response{Content: "10 月 1 日未被本次预报覆盖，天气未知。"}, nil
	})
	h := setup(t, AgentRunner{Client: model, Tools: reg, MaxMessages: 20, MaxIterations: 3})
	ss := h.newSession()
	var run Run
	h.request("POST", "/api/sessions/"+ss.ID+"/runs", startBody("杭州 10 月 1 日天气如何？", "dated-weather", 1), 202, &run)
	if r := h.wait(run.ID); r.Status != "completed" {
		t.Fatalf("weather run failed: %+v", r)
	}
	h.request("GET", "/api/sessions/"+ss.ID, nil, 200, &ss)
	if len(ss.Messages) != 4 || ss.Messages[2].ToolCallID != "weather-date" || !strings.Contains(ss.Messages[3].Content, "天气未知") {
		t.Fatalf("complete weather turn was not persisted: %+v", ss.Messages)
	}
}

func TestTripWeatherAlternativeEvidenceIsPersisted(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/geo/v2/city/lookup" {
			fmt.Fprint(w, `{"code":"200","location":[{"id":"101210101"}]}`)
			return
		}
		fmt.Fprintf(w, `{"code":"200","updateTime":%q,"daily":[{"fxDate":"2026-09-22","tempMax":"25","tempMin":"20","textDay":"雨","precip":"4.1"}]}`, time.Now().UTC().Add(-time.Minute).Format(time.RFC3339))
	}))
	defer provider.Close()
	reg := tool.NewRegistry()
	reg.Register(builtin.NewWeather("test-key", provider.URL))
	calls := 0
	model := modelFunc(func(ctx context.Context, messages []llm.Message, defs []llm.ToolDef) (*llm.Response, error) {
		calls++
		if calls == 1 {
			tc := llm.ToolCall{ID: "weather-alternative", Type: "function"}
			tc.Function.Name = "weather"
			tc.Function.Arguments = `{"city":"杭州","date":"2026-09-22"}`
			return &llm.Response{ToolCalls: []llm.ToolCall{tc}}, nil
		}
		evidence := messages[len(messages)-1]
		if evidence.Role != llm.RoleTool || !strings.Contains(evidence.Content, "预报降水量：4.1 mm") || !strings.Contains(evidence.Content, "应用规则，非气象观测") {
			t.Errorf("missing alternative provenance: %+v", evidence)
		}
		return &llm.Response{Content: "可考虑室内备选，开放情况待核实。"}, nil
	})
	h := setup(t, AgentRunner{Client: model, Tools: reg, MaxMessages: 20, MaxIterations: 3})
	ss := h.newSession()
	var run Run
	h.request("POST", "/api/sessions/"+ss.ID+"/runs", startBody("杭州 9 月 22 日有室内备选吗？", "weather-alternative", 1), 202, &run)
	if r := h.wait(run.ID); r.Status != "completed" {
		t.Fatalf("weather run failed: %+v", r)
	}
	h.request("GET", "/api/sessions/"+ss.ID, nil, 200, &ss)
	if len(ss.Messages) != 4 || ss.Messages[2].ToolCallID != "weather-alternative" || !strings.Contains(ss.Messages[2].Content, "室内展览或室内阅读休息") {
		t.Fatalf("alternative evidence lost: %+v", ss.Messages)
	}
}
