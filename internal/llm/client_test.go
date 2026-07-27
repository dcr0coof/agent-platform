package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIClient_Chat_TextResponse(t *testing.T) {
	// mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openaiChatResponse{
			Choices: []struct {
				Message struct {
					Content   string     `json:"content"`
					ToolCalls []ToolCall `json:"tool_calls"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			}{
				{
					Message: struct {
						Content   string     `json:"content"`
						ToolCalls []ToolCall `json:"tool_calls"`
					}{Content: "你好！有什么可以帮你的？"},
					FinishReason: "stop",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOpenAI("fake-key", server.URL, "test-model", 100, 0.5)
	resp, err := client.Chat(context.Background(), []Message{
		{Role: RoleUser, Content: "你好"},
	}, nil)

	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
	if resp.Content != "你好！有什么可以帮你的？" {
		t.Errorf("unexpected content: %s", resp.Content)
	}
	if len(resp.ToolCalls) != 0 {
		t.Errorf("expected 0 tool calls, got %d", len(resp.ToolCalls))
	}
}

func TestOpenAIClient_Chat_ToolCallResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openaiChatResponse{
			Choices: []struct {
				Message struct {
					Content   string     `json:"content"`
					ToolCalls []ToolCall `json:"tool_calls"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			}{
				{
					Message: struct {
						Content   string     `json:"content"`
						ToolCalls []ToolCall `json:"tool_calls"`
					}{
						ToolCalls: []ToolCall{
							{
								ID:   "call_1",
								Type: "function",
								Function: struct {
									Name      string `json:"name"`
									Arguments string `json:"arguments"`
								}{Name: "calculator", Arguments: `{"expression":"2+3"}`},
							},
						},
					},
					FinishReason: "tool_calls",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOpenAI("fake-key", server.URL, "test-model", 100, 0.5)
	resp, err := client.Chat(context.Background(), []Message{
		{Role: RoleUser, Content: "2+3等于几"},
	}, []ToolDef{})

	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Function.Name != "calculator" {
		t.Errorf("unexpected tool name: %s", resp.ToolCalls[0].Function.Name)
	}
}
