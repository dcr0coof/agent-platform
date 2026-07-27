package agent

import (
	"context"
	"encoding/json"
	"fmt"

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
}

// New 创建 Agent
func New(name string, llmClient llm.Client, mem memory.Memory, tools *tool.Registry, maxIter int) *Agent {
	return &Agent{
		name:      name,
		llmClient: llmClient,
		memory:    mem,
		tools:     tools,
		maxIter:   maxIter,
	}
}

// SetSystemPrompt 设置系统提示词
func (a *Agent) SetSystemPrompt(prompt string) {
	a.memory.SetSystem(prompt)
}

// Run 处理一条用户消息，返回最终文本回复
func (a *Agent) Run(ctx context.Context, userMessage string) (string, error) {
	// 追加用户消息
	a.memory.Add(llm.Message{Role: llm.RoleUser, Content: userMessage})

	// 构建工具定义列表
	toolDefs := buildToolDefs(a.tools)

	for i := 0; i < a.maxIter; i++ {
		messages := a.memory.All()

		resp, err := a.llmClient.Chat(ctx, messages, toolDefs)
		if err != nil {
			return "", fmt.Errorf("LLM 调用失败: %w", err)
		}

		// LLM 返回工具调用
		if len(resp.ToolCalls) > 0 {
			// 记录 assistant 的工具调用请求
			a.memory.Add(llm.Message{
				Role:      llm.RoleAssistant,
				ToolCalls: resp.ToolCalls,
			})

			// 逐个执行工具
			for _, tc := range resp.ToolCalls {
				toolResult, err := executeTool(ctx, a.tools, tc)
				if err != nil {
					toolResult = fmt.Sprintf("工具执行失败: %v", err)
				}
				a.memory.Add(llm.Message{
					Role:       llm.RoleTool,
					Content:    toolResult,
					ToolCallID: tc.ID,
				})
			}
			// 继续循环，让 LLM 处理工具结果
			continue
		}

		// LLM 返回纯文本
		a.memory.Add(llm.Message{Role: llm.RoleAssistant, Content: resp.Content})
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
func executeTool(ctx context.Context, reg *tool.Registry, tc llm.ToolCall) (string, error) {
	t, err := reg.Get(tc.Function.Name)
	if err != nil {
		return "", err
	}

	var params map[string]interface{}
	if tc.Function.Arguments != "" {
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &params); err != nil {
			return "", fmt.Errorf("参数解析失败: %w", err)
		}
	}

	return t.Execute(ctx, params)
}
