package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIHTTPStatus(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		w.Write([]byte("rate limited"))
	}))
	defer s.Close()
	_, err := NewOpenAI("key", s.URL, "model", 100, 0).Chat(context.Background(), nil, nil)
	if err == nil || !strings.Contains(err.Error(), "429") {
		t.Fatalf("missing HTTP status: %v", err)
	}
}

func TestOpenAIResponseFailures(t *testing.T) {
	for _, body := range []string{
		`{"error":{"message":"bad request","type":"invalid"}}`,
		`{"choices":[]}`, `not json`,
		`{"choices":[{"message":{"content":"partial"},"finish_reason":"length"}]}`,
		`{"choices":[{"message":{"content":"filtered"},"finish_reason":"content_filter"}]}`,
		strings.Repeat("x", (4<<20)+1),
	} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
		_, err := NewOpenAI("key", s.URL, "model", 100, 0).Chat(context.Background(), nil, nil)
		s.Close()
		if err == nil {
			t.Fatal("bad response accepted")
		}
	}
}

func TestOpenAICancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewOpenAI("key", "http://127.0.0.1:1", "model", 100, 0).Chat(ctx, nil, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}

func TestOpenAIRequestValues(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("bad path: %s", r.URL.Path)
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if v, ok := body["temperature"]; !ok || v != float64(0) {
			t.Errorf("zero temperature omitted: %v", body)
		}
		msgs := body["messages"].([]interface{})
		if _, ok := msgs[0].(map[string]interface{})["content"]; !ok {
			t.Error("empty tool content omitted")
		}
		w.Write([]byte(`{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer s.Close()
	_, err := NewOpenAI("key", s.URL+"/", "model", 100, 0).Chat(context.Background(), []Message{{Role: RoleTool, ToolCallID: "a"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
}
