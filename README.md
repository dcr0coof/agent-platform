# agent-platform

从零构建的通用 AI Agent 平台 —— 对话 + 工具调用 + 记忆。

**技术栈：** Go + TypeScript（微信小程序）

## 快速开始

```bash
# 设置 LLM API
export AGENT_LLM_API_KEY="your-deepseek-api-key"
export AGENT_LLM_BASE_URL="https://api.deepseek.com/v1"

# 运行 CLI 对话
go run ./cmd/agent
```

## 项目结构

```
agent-platform/
├── cmd/agent/          # CLI 入口
├── internal/
│   ├── agent/          # Agent 核心循环
│   ├── llm/            # LLM 客户端
│   ├── tool/           # 工具系统
│   ├── memory/         # 记忆管理
│   └── config/         # 配置加载
└── configs/            # 默认配置
```

## License

MIT
