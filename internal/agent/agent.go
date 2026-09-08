package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dcr0coof/agent-platform/internal/llm"
	"github.com/dcr0coof/agent-platform/internal/memory"
	"github.com/dcr0coof/agent-platform/internal/tool"
)

// Agent 通用 AI Agent
type Agent struct {
	name      string
	llmClient llm.Client
	memory    memory.Memory
	tools     *tool.Registry
	maxIter   int
	runGate   chan struct{}
}

// New 创建 Agent
func New(name string, llmClient llm.Client, mem memory.Memory, tools *tool.Registry, maxIter int) *Agent {
	if maxIter <= 0 {
		maxIter = 10
	}
	return &Agent{
		name:      name,
		llmClient: llmClient,
		memory:    mem,
		tools:     tools,
		maxIter:   maxIter,
		runGate:   make(chan struct{}, 1),
	}
}

// SetSystemPrompt 设置系统提示词
func (a *Agent) SetSystemPrompt(prompt string) {
	a.runGate <- struct{}{}
	defer func() { <-a.runGate }()
	a.memory.SetSystem(prompt)
}

// Clear 清空当前会话。与 Run 串行，避免运行结束时写回已清除的历史。
func (a *Agent) Clear() {
	a.runGate <- struct{}{}
	defer func() { <-a.runGate }()
	a.memory.Clear()
}

// Run 处理一条用户消息，返回最终文本回复
func (a *Agent) Run(ctx context.Context, userMessage string) (string, error) {
	if strings.TrimSpace(userMessage) == "" {
		return "", fmt.Errorf("用户消息不能为空")
	}
	select {
	case a.runGate <- struct{}{}:
		defer func() { <-a.runGate }()
	case <-ctx.Done():
		return "", ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	// 仅在本回合成功后提交记忆。失败、取消和迭代耗尽不污染下一次对话。
	messages := a.memory.All()
	start := len(messages)
	messages = append(messages, llm.Message{Role: llm.RoleUser, Content: userMessage})

	// 构建工具定义列表
	toolDefs := buildToolDefs(a.tools)

	for i := 0; i < a.maxIter; i++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		resp, err := a.llmClient.Chat(ctx, messages, toolDefs)
		if err != nil {
			return "", fmt.Errorf("LLM 调用失败: %w", err)
		}
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if resp == nil {
			return "", fmt.Errorf("LLM 返回空响应")
		}

		// LLM 返回工具调用
		if len(resp.ToolCalls) > 0 {
			ids := make(map[string]bool, len(resp.ToolCalls))
			for _, tc := range resp.ToolCalls {
				if tc.ID == "" || ids[tc.ID] || tc.Type != "function" || tc.Function.Name == "" {
					return "", fmt.Errorf("LLM 返回无效工具调用")
				}
				ids[tc.ID] = true
			}
			// 记录 assistant 的工具调用请求
			messages = append(messages, llm.Message{
				Role:      llm.RoleAssistant,
				Content:   resp.Content,
				ToolCalls: resp.ToolCalls,
			})

			// 逐个执行工具
			for _, tc := range resp.ToolCalls {
				if err := ctx.Err(); err != nil {
					return "", err
				}
				toolResult, err := executeTool(ctx, a.tools, tc)
				if ctx.Err() != nil {
					return "", ctx.Err()
				}
				if err != nil {
					toolResult = fmt.Sprintf("工具执行失败: %v", err)
				}
				messages = append(messages, llm.Message{
					Role:       llm.RoleTool,
					Content:    toolResult,
					ToolCallID: tc.ID,
				})
			}
			// 继续循环，让 LLM 处理工具结果
			continue
		}

		// LLM 返回纯文本
		if strings.TrimSpace(resp.Content) == "" {
			return "", fmt.Errorf("LLM 返回空文本且没有工具调用")
		}
		messages = append(messages, llm.Message{Role: llm.RoleAssistant, Content: resp.Content})
		for _, msg := range messages[start:] {
			a.memory.Add(msg)
		}
		return resp.Content, nil
	}

	return "", fmt.Errorf("达到最大迭代次数 %d，工具调用未收敛", a.maxIter)
}

// buildToolDefs 将注册表中的工具转为 LLM 需要的 ToolDef 列表
func buildToolDefs(reg *tool.Registry) []llm.ToolDef {
	tools := reg.List()
	defs := make([]llm.ToolDef, 0, len(tools))
	for _, t := range tools {
		def := llm.ToolDef{Type: "function"}
		def.Function.Name = t.Name()
		def.Function.Description = t.Description()
		def.Function.Parameters = t.Parameters()
		defs = append(defs, def)
	}
	return defs
}

// executeTool 执行一个工具调用
func executeTool(ctx context.Context, reg *tool.Registry, tc llm.ToolCall) (result string, err error) {
	defer func() {
		if recover() != nil {
			result, err = "", fmt.Errorf("工具 %s 执行异常", tc.Function.Name)
		}
	}()
	t, err := reg.Get(tc.Function.Name)
	if err != nil {
		return "", err
	}

	params := make(map[string]interface{})
	if tc.Function.Arguments != "" {
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &params); err != nil {
			return "", fmt.Errorf("参数解析失败: %w", err)
		}
		if params == nil {
			return "", fmt.Errorf("工具参数必须是 JSON 对象")
		}
	}

	return t.Execute(ctx, params)
}
