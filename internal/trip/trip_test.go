package trip

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dcr0coof/agent-platform/internal/llm"
	"github.com/dcr0coof/agent-platform/internal/tool"
)

type runnerFunc func(context.Context, Session, string, func(string, string) error) ([]llm.Message, error)

func (f runnerFunc) Execute(ctx context.Context, s Session, input string, e func(string, string) error) ([]llm.Message, error) {
	return f(ctx, s, input, e)
}

type harness struct {
	server  *httptest.Server
	client  *http.Client
	store   *Store
	service *Service
	t       *testing.T
}

func setup(t *testing.T, r Runner) *harness {
	t.Helper()
	store, err := OpenStore(filepath.Join(t.TempDir(), "trips.db"))
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(store, r, time.Second*2)
	server := httptest.NewServer((&HTTP{Service: service, Demo: true, WebDir: t.TempDir()}).Handler())
	jar, _ := cookiejar.New(nil)
	h := &harness{server: server, client: &http.Client{Jar: jar, Timeout: 3 * time.Second}, store: store, service: service, t: t}
	t.Cleanup(func() { service.Close(); server.Close(); store.Close() })
	h.request("GET", "/api/bootstrap", nil, 200, nil)
	return h
}
func (h *harness) request(method, path string, data interface{}, status int, result interface{}) {
	h.t.Helper()
	var body io.Reader
	if data != nil {
		body = bytes.NewBufferString(encode(data))
	}
	req, _ := http.NewRequest(method, h.server.URL+path, body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.client.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		h.t.Fatal(err)
	}
	if resp.StatusCode != status {
		h.t.Fatalf("%s %s: status %d, want %d: %s", method, path, resp.StatusCode, status, raw)
	}
	if result != nil {
		if err = json.Unmarshal(raw, result); err != nil {
			h.t.Fatal(err)
		}
	}
}
func (h *harness) newSession() Session {
	h.t.Helper()
	var s Session
	h.request("POST", "/api/sessions", sessionInput{Title: "杭州周末", Constraints: Constraints{}}, 201, &s)
	return s
}
func (h *harness) wait(id string) Run {
	h.t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var r Run
		h.request("GET", "/api/runs/"+id, nil, 200, &r)
		if r.Terminal() {
			return r
		}
		time.Sleep(time.Millisecond * 5)
	}
	h.t.Fatal("run did not terminate")
	return Run{}
}
func startBody(message, key string, revision int) map[string]interface{} {
	return map[string]interface{}{"message": message, "request_id": key, "revision": revision}
}

func TestSessionListDoesNotDecodeHistory(t *testing.T) {
	h := setup(t, DemoRunner{})
	s := h.newSession()
	// A damaged history must not hide the navigation metadata for all trips.
	if _, err := h.store.db.Exec("UPDATE sessions SET messages_json=? WHERE id=?", "invalid history", s.ID); err != nil {
		t.Fatal(err)
	}
	var list []Session
	h.request("GET", "/api/sessions", nil, 200, &list)
	if len(list) != 1 || list[0].ID != s.ID || list[0].Title != s.Title || list[0].Revision != s.Revision || list[0].Messages != nil {
		t.Fatalf("unexpected summaries: %+v", list)
	}
	// Detail reads still surface corruption; listing does not repair or erase it.
	h.request("GET", "/api/sessions/"+s.ID, nil, 500, nil)
	var raw string
	if err := h.store.db.QueryRow("SELECT messages_json FROM sessions WHERE id=?", s.ID).Scan(&raw); err != nil || raw != "invalid history" {
		t.Fatalf("history changed: %q, %v", raw, err)
	}
}

func BenchmarkListLargeHistory(b *testing.B) {
	store, err := OpenStore(filepath.Join(b.TempDir(), "trips.db"))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { store.Close() })
	s, err := store.Create("benchmark-owner", "large history", Constraints{})
	if err != nil {
		b.Fatal(err)
	}
	history := encode([]llm.Message{{Role: llm.RoleUser, Content: strings.Repeat("x", 1<<20)}})
	if _, err = store.db.Exec("UPDATE sessions SET messages_json=? WHERE id=?", history, s.ID); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		list, err := store.List("benchmark-owner")
		if err != nil || len(list) != 1 || list[0].Messages != nil {
			b.Fatalf("invalid list: %v, %v", list, err)
		}
	}
}

func TestConversationPersistenceAndIdempotency(t *testing.T) {
	h := setup(t, DemoRunner{Delay: time.Millisecond})
	s := h.newSession()
	if s.ID == "" || s.Revision != 1 || len(s.Messages) != 0 {
		t.Fatal("invalid new session")
	}
	c := Constraints{Origin: "上海", Destination: "杭州", StartDate: "2026-10-01", EndDate: "2026-10-02", Travelers: 2, Budget: 1500, BudgetScope: "all", Preferences: "少走路"}
	h.request("PUT", "/api/sessions/"+s.ID, sessionInput{Title: "轻松杭州", Constraints: c, Revision: s.Revision}, 200, &s)
	if s.Constraints.Origin != "上海" || s.Revision != 2 {
		t.Fatal("constraints not saved")
	}
	h.request("PUT", "/api/sessions/"+s.ID, sessionInput{Constraints: c, Revision: 1}, 409, nil)
	var r Run
	body := startBody("检查我的出行条件", "test-request-01", s.Revision)
	h.request("POST", "/api/sessions/"+s.ID+"/runs", body, 202, &r)
	completed := h.wait(r.ID)
	if completed.Status != "completed" {
		t.Fatal(completed)
	}
	var retry Run
	h.request("POST", "/api/sessions/"+s.ID+"/runs", body, 202, &retry)
	if retry.ID != r.ID {
		t.Fatal("retry created another run")
	}
	h.request("POST", "/api/sessions/"+s.ID+"/runs", startBody("changed input", "test-request-01", s.Revision), 409, nil)
	h.request("GET", "/api/sessions/"+s.ID, nil, 200, &s)
	if len(s.Messages) != 2 || !strings.Contains(s.Messages[1].Content, "上海 → 杭州") || !strings.Contains(s.Messages[1].Content, "少走路") {
		t.Fatal("successful turn not persisted")
	}
	if s.Revision != 3 {
		t.Fatal("history commit did not increment revision")
	}
}

func TestCancelledRunsLeaveHistoryAndCannotWriteMoreEvents(t *testing.T) {
	started := make(chan struct{}, 4)
	h := setup(t, runnerFunc(func(ctx context.Context, s Session, input string, emit func(string, string) error) ([]llm.Message, error) {
		if err := emit("tool.started", "waiting"); err != nil {
			return nil, err
		}
		started <- struct{}{}
		<-ctx.Done()
		return nil, ctx.Err()
	}))
	s1, s2 := h.newSession(), h.newSession()
	var r1, r2 Run
	h.request("POST", "/api/sessions/"+s1.ID+"/runs", startBody("first", "cancel-request-1", 1), 202, &r1)
	<-started
	h.request("POST", "/api/sessions/"+s1.ID+"/runs", startBody("duplicate", "cancel-request-2", 1), 409, nil)
	h.request("PUT", "/api/sessions/"+s1.ID, sessionInput{Title: "change while running", Revision: 1}, 409, nil)
	h.request("POST", "/api/sessions/"+s2.ID+"/runs", startBody("second session", "cancel-request-3", 1), 202, &r2)
	<-started // A different session progresses while the first is blocked.
	h.request("POST", "/api/runs/"+r1.ID+"/cancel", nil, 200, &r1)
	if r1.Status != "cancelled" {
		t.Fatal("cancel did not persist")
	}
	h.request("GET", "/api/sessions/"+s1.ID, nil, 200, &s1)
	if len(s1.Messages) != 0 || s1.ActiveRun != nil {
		t.Fatal("cancel polluted session")
	}
	if err := h.store.AppendEvent(r1.ID, "late", "should not exist"); !errors.Is(err, context.Canceled) {
		t.Fatal("terminal run accepted late events")
	}
	h.request("POST", "/api/runs/"+r2.ID+"/cancel", nil, 200, nil)
	h.request("POST", "/api/runs/"+r1.ID+"/cancel", nil, 200, nil)
}

func TestSSEReplayAndWorkspaceIsolation(t *testing.T) {
	var count atomic.Int32
	h := setup(t, runnerFunc(func(ctx context.Context, s Session, input string, emit func(string, string) error) ([]llm.Message, error) {
		count.Add(1)
		if err := emit("context.ready", "ready"); err != nil {
			return nil, err
		}
		return []llm.Message{{Role: llm.RoleUser, Content: input}, {Role: llm.RoleAssistant, Content: "reply"}}, nil
	}))
	s := h.newSession()
	var r Run
	h.request("POST", "/api/sessions/"+s.ID+"/runs", startBody("hello", "stream-request-1", 1), 202, &r)
	done := h.wait(r.ID)
	req, _ := http.NewRequest("GET", h.server.URL+"/api/runs/"+r.ID+"/events?after=0", nil)
	req.Header.Set("Last-Event-ID", "1")
	response, err := h.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if !strings.HasPrefix(response.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatal("not SSE")
	}
	scan := bufio.NewScanner(response.Body)
	seqs := []int{}
	for scan.Scan() {
		if strings.HasPrefix(scan.Text(), "data: ") {
			var e Event
			if err = json.Unmarshal([]byte(strings.TrimPrefix(scan.Text(), "data: ")), &e); err != nil {
				t.Fatal(err)
			}
			seqs = append(seqs, e.Seq)
		}
	}
	if err = scan.Err(); err != nil {
		t.Fatal(err)
	}
	if len(seqs) != len(done.Events)-1 || seqs[0] != 2 || count.Load() != 1 {
		t.Fatalf("replay lost events or re-executed: %v / %d", seqs, count.Load())
	}
	jar, _ := cookiejar.New(nil)
	h.client.Jar = jar
	h.request("GET", "/api/bootstrap", nil, 200, nil)
	h.request("GET", "/api/sessions/"+s.ID, nil, 404, nil)
	h.request("GET", "/api/runs/"+r.ID, nil, 404, nil)
	h.request("GET", "/api/runs/"+r.ID+"/events", nil, 404, nil)
	h.request("POST", "/api/runs/"+r.ID+"/cancel", nil, 404, nil)
	var sessions []Session
	h.request("GET", "/api/sessions", nil, 200, &sessions)
	if len(sessions) != 0 {
		t.Fatal("workspace data leaked")
	}
}

func TestStoreRestartPreservesHistoryAndFailsInterruptedRun(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trips.db")
	s, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	ss, err := s.Create("owner", "trip", Constraints{Destination: "杭州"})
	if err != nil {
		t.Fatal(err)
	}
	r, _, err := s.Start("owner", ss.ID, "first", "restart-1", 1)
	if err != nil {
		t.Fatal(err)
	}
	history := []llm.Message{{Role: llm.RoleUser, Content: "first"}, {Role: llm.RoleAssistant, Content: "reply"}}
	if err = s.Finish(r.ID, "completed", "done", history); err != nil {
		t.Fatal(err)
	}
	pending, _, err := s.Start("owner", ss.ID, "second", "restart-2", 2)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	restored, err := s.Get("owner", ss.ID)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Constraints.Destination != "杭州" || len(restored.Messages) != 2 {
		t.Fatal("lost saved session")
	}
	interrupted, err := s.Run("owner", pending.ID)
	if err != nil {
		t.Fatal(err)
	}
	if interrupted.Status != "failed" || interrupted.Events[len(interrupted.Events)-1].Type != "run.failed" {
		t.Fatal("interrupted run not recovered")
	}
}

func TestHTTPRejectsInvalidAndCrossOriginRequests(t *testing.T) {
	h := setup(t, DemoRunner{})
	s := h.newSession()
	for _, c := range []Constraints{{StartDate: "bad"}, {StartDate: "2026-10-02", EndDate: "2026-10-01"}, {Travelers: -1}, {Budget: -2}, {BudgetScope: "pretend"}} {
		h.request("PUT", "/api/sessions/"+s.ID, sessionInput{Constraints: c, Revision: 1}, 400, nil)
	}
	h.request("POST", "/api/sessions/"+s.ID+"/runs", startBody(" ", "valid-request", 1), 400, nil)
	req, _ := http.NewRequest("POST", h.server.URL+"/api/sessions", strings.NewReader(`{"title":"bad"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://evil.example")
	response, err := h.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 403 {
		t.Fatal("cross-origin mutation accepted")
	}
	req, _ = http.NewRequest("GET", h.server.URL+"/api/bootstrap", nil)
	req.Host = "evil.example"
	response, err = h.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 403 {
		t.Fatal("untrusted Host accepted")
	}
}

type modelFunc func(context.Context, []llm.Message, []llm.ToolDef) (*llm.Response, error)

func (f modelFunc) Chat(ctx context.Context, m []llm.Message, t []llm.ToolDef) (*llm.Response, error) {
	return f(ctx, m, t)
}
func TestAgentRunnerUsesConstraintsAndPreservesFullToolTurn(t *testing.T) {
	calls := 0
	model := modelFunc(func(ctx context.Context, m []llm.Message, defs []llm.ToolDef) (*llm.Response, error) {
		calls++
		if !strings.Contains(m[0].Content, "少走路") {
			t.Fatal("missing confirmed preferences")
		}
		if calls == 1 {
			tc := llm.ToolCall{ID: "a", Type: "function"}
			tc.Function.Name = "unknown"
			tc.Function.Arguments = "{}"
			return &llm.Response{ToolCalls: []llm.ToolCall{tc}}, nil
		}
		if len(m) < 4 || m[len(m)-1].Role != llm.RoleTool {
			t.Fatal("tool chain lost")
		}
		return &llm.Response{Content: "ready"}, nil
	})
	runner := AgentRunner{Client: model, Tools: tool.NewRegistry(), MaxMessages: 2, MaxIterations: 3}
	stages := []string{}
	added, err := runner.Execute(context.Background(), Session{Constraints: Constraints{Preferences: "少走路"}}, "hello", func(kind, detail string) error { stages = append(stages, kind); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(added) != 4 || added[0].Role != llm.RoleUser || added[3].Content != "ready" {
		t.Fatalf("complete turn lost: %+v", added)
	}
	if len(stages) < 5 {
		t.Fatal("observable steps missing")
	}
}

func TestRunnerFailureIsRecordedWithoutExposingProviderError(t *testing.T) {
	h := setup(t, runnerFunc(func(context.Context, Session, string, func(string, string) error) ([]llm.Message, error) {
		return nil, errors.New("private credential reflected by provider")
	}))
	s := h.newSession()
	var r Run
	h.request("POST", "/api/sessions/"+s.ID+"/runs", startBody("hello", "failure-request", 1), 202, &r)
	r = h.wait(r.ID)
	if r.Status != "failed" || strings.Contains(r.Error, "private credential") {
		t.Fatal("provider failure leaked")
	}
	h.request("GET", "/api/sessions/"+s.ID, nil, 200, &s)
	if len(s.Messages) != 0 {
		t.Fatal("failure polluted history")
	}
}
