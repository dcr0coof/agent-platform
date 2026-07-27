package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/dcr0coof/agent-platform/internal/agent"
	"github.com/dcr0coof/agent-platform/internal/config"
	"github.com/dcr0coof/agent-platform/internal/llm"
	"github.com/dcr0coof/agent-platform/internal/memory"
	"github.com/dcr0coof/agent-platform/internal/tool"
	"github.com/dcr0coof/agent-platform/internal/tool/builtin"
)

func main() {
	// 加载配置
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	if cfg.LLM.APIKey == "" {
		fmt.Fprintln(os.Stderr, "错误: 请设置环境变量 AGENT_LLM_API_KEY")
		fmt.Fprintln(os.Stderr, "  export AGENT_LLM_API_KEY=\"your-deepseek-api-key\"")
		os.Exit(1)
	}

	// 创建组件
	llmClient := llm.NewOpenAI(
		cfg.LLM.APIKey,
		cfg.LLM.BaseURL,
		cfg.LLM.Model,
		cfg.LLM.MaxTokens,
		cfg.LLM.Temperature,
	)

	reg := tool.NewRegistry()
	reg.Register(&builtin.Calculator{})
	reg.Register(&builtin.DateTime{})

	// 天气工具：从配置加载 API Key
	if cfg.Weather.APIKey != "" {
		reg.Register(builtin.NewWeather(cfg.Weather.APIKey))
	}

	mem := memory.NewBuffer(cfg.Memory.MaxMessages)

	ag := agent.New(cfg.Agent.Name, llmClient, mem, reg, cfg.Agent.MaxIterations)
	ag.SetSystemPrompt("你是一个有用的 AI 助手。你可以使用计算器工具做数学计算，使用 datetime 工具获取当前时间，使用 weather 工具查询天气。回答简洁，用中文。")

	fmt.Printf("🤖 %s 已启动 (模型: %s)\n", cfg.Agent.Name, cfg.LLM.Model)
	fmt.Println("输入消息开始对话，输入 /exit 退出，/clear 清空对话")
	fmt.Println(strings.Repeat("─", 50))

	scanner := bufio.NewScanner(os.Stdin)
	ctx := context.Background()

	for {
		fmt.Print("\n你> ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		switch input {
		case "/exit":
			fmt.Println("再见！")
			return
		case "/clear":
			mem.Clear()
			fmt.Println("对话已清空")
			continue
		}

		fmt.Print("\n助手> ")
		result, err := ag.Run(ctx, input)
		if err != nil {
			fmt.Printf("错误: %v\n", err)
			continue
		}
		fmt.Println(result)
	}
}
