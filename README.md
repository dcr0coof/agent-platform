# agent-platform

用 Go 实现的 AI Agent 实战项目：命令行多轮对话、工具调用、短期记忆。默认连接 DeepSeek，也可以配置提供 Chat Completions 工具调用协议的服务。

当前已实现 CLI 核心引擎及天气工具，以及「行迹」Vue 3 + TypeScript 出行工作台：可编辑约束、SQLite 持久会话、SSE 执行事件、取消和断线续读。攻略 RAG、token 预算与摘要、地图和行程生成仍在规划中。

## Web 工作台

需要 Go 1.22.12+、Node.js 22.12+。从项目根目录运行：

```bash
npm --prefix web ci
npm --prefix web run build
go run ./cmd/server -demo
```

打开 <http://127.0.0.1:8080>。演示模式明确标记为规则回复，不访问模型或天气服务；可体验创建出行、编辑条件、发送、取消和刷新恢复。真实模型模式设置下文的 `AGENT_LLM_API_KEY` 后运行 `go run ./cmd/server`；天气沿用下文配置。

本版仅绑定本机地址，数据默认保存在 `data/trips.db`，每个数据库文件只运行一个服务实例。浏览器 Cookie 隔离数据，不是多用户账号系统；清除 Cookie 后无法通过界面访问原会话。详细启动、API、测试和限制见 [工作台说明](docs/trip-workspace.md)。

## 快速开始

需要 Go 1.22.12 或更高版本，以及可用的模型 API Key。从项目根目录启动。

PowerShell：

```powershell
$env:AGENT_LLM_API_KEY = "your-api-key"
go run ./cmd/agent
```

Bash：

```bash
export AGENT_LLM_API_KEY="your-api-key"
go run ./cmd/agent
```

输入 `帮我算 (123+456)*7` 或 `现在几点了`。`/clear` 清空历史并保留系统提示词，`/exit`、EOF 或 Ctrl+C 退出。

```bash
go run ./cmd/agent -config configs/config.yaml -timeout 2m
go run ./cmd/agent -h
go build -o bin/agent ./cmd/agent
```

`-timeout` 是整轮对话的时间预算，含所有模型与工具请求，默认 2 分钟。单次模型请求另有 60 秒上限，天气请求 15 秒。单行输入最多约 1 MiB，过长会明确报错。

## 配置

优先级：环境变量 > YAML > 默认值。配置文件缺失时使用默认值和环境变量；文件存在但格式错误、字段拼错或数值非法时启动失败。模型 API Key 必填。

| YAML 字段 | 环境变量 | 默认值 / 含义 |
| --- | --- | --- |
| `llm.api_key` | `AGENT_LLM_API_KEY` | 无；建议通过环境变量设置 |
| `llm.base_url` | `AGENT_LLM_BASE_URL` | `https://api.deepseek.com/v1`；不包含 `/chat/completions` |
| `llm.model` | `AGENT_LLM_MODEL` | `deepseek-chat` |
| `llm.max_tokens` | — | `2000`，必须大于 0 |
| `llm.temperature` | — | `0.7`，支持显式设置 `0`，范围 0～2 |
| `agent.max_iterations` | — | `10`，一轮最多调用模型的次数，含最终回答 |
| `agent.name` | — | `default`，CLI 显示名称 |
| `memory.max_messages` | — | `20`，按完整用户回合裁剪的消息数目标 |
| `memory.enable_summary` | — | `false`；摘要未实现，设为 `true` 明确报错 |
| `memory.summary_trigger` | — | `30`，保留的旧字段，当前不生效 |
| `weather.api_key` | `AGENT_WEATHER_API_KEY` | 可选，设置后启用天气工具 |
| `weather.base_url` | `AGENT_WEATHER_BASE_URL` | 启用天气时必填，专属 API Host 的完整 HTTPS URL |

切换模型时，`base_url`、`model` 和密钥需匹配该服务；各服务对模型参数和工具调用的支持可能不同。

### 启用天气

```powershell
$env:AGENT_WEATHER_API_KEY = "your-qweather-key"
$env:AGENT_WEATHER_BASE_URL = "https://your-host.qweatherapi.com"
go run ./cmd/agent
```

API Host 从和风天气控制台获取。官方说明旧共享域名从 2026 年起逐步停用，因此本项目使用专属域名配置。凭证放在 `X-QW-Api-Key` 请求头中。参考：[API Host](https://dev.qweather.com/en/docs/configuration/api-host/)、[请求配置](https://dev.qweather.com/en/docs/configuration/api-config/)。

天气支持城市名或 Location ID，以及 `now`（实况）和 `forecast`（3 天预报）。沿用城市版接口：`/geo/v2/city/lookup`、`/v7/weather/now`、`/v7/weather/3d`。同名城市选择服务返回的第一项，可提供更具体的城市名或 ID。

## 运行行为

- 每次 `Run` 在局部历史中完成“用户 → 模型 → 工具 → 模型”循环，成功后才提交本轮消息。请求失败、取消或迭代耗尽时保留之前的历史。
- 记忆按完整用户回合淘汰。单个回合超过 `max_messages` 时完整保留，下一回合到来后可以整体淘汰；该配置是软限制，不是 token 上限。
- 同一个 Agent 的 `Run` 串行执行，等待锁时支持取消。`SetSystemPrompt` 和 `Clear` 与运行互斥；Web 为每轮创建独立 Agent 和 Memory，从所属会话加载完整回合窗口。
- 工具失败、未知工具、无效参数和工具 panic 会转换为工具结果，让模型有机会解释或纠正。无效工具调用 ID、空回答和被截断的回答作为请求失败处理。
- 工具按模型返回顺序执行。失败回合不写记忆，但已经执行的工具副作用不能回滚；未来增加写入类工具时需要幂等性设计。
- HTTP 错误明确报告状态码；LLM 响应限制为 4 MiB，天气解压后的响应限制为 1 MiB。请求不自动重试，不跟随重定向。
- CLI 会话仅保存在内存中，退出后丢失；Web 保存完整成功回合，运行事件单独持久化，失败或取消不污染聊天历史。计算器使用浮点数，支持四则运算、正负号及括号，不保证任意精度。

## 项目结构

```text
cmd/agent/             CLI 入口和端到端测试
cmd/server/            本机 HTTP 服务入口
internal/agent/        对话循环、工具调度和会话互斥
internal/trip/         会话、约束、SQLite、运行生命周期和 SSE
internal/llm/          消息协议、客户端接口和 HTTP 实现
internal/tool/         工具接口、注册表
internal/tool/builtin/ 计算器、当前日期时间、和风天气
internal/memory/       保留完整回合的内存窗口
internal/config/       YAML、环境变量和启动校验
configs/              默认配置
web/                  Vue 3 + TypeScript 工作台与 Playwright 测试
docs/                 代码审查、优化说明和后续路线
.github/workflows/     Windows / Linux 检查
```

扩展工具：实现 `tool.Tool` 的四个方法，在入口注册，并用测试覆盖成功和失败行为。`Execute` 应响应 `ctx` 取消，不能假定模型参数一定符合 Schema。`Parameters` 必须返回可 JSON 编码的 Schema。

## 验证

```bash
go test ./... -cover
go vet ./...
go build ./...
go test -race ./...
```

测试通过本地模拟 HTTP 服务运行，无需真实密钥，也不消耗模型额度。CLI 测试会编译临时可执行文件，覆盖工具调用、清空、EOF、帮助、超时参数校验和超长输入。`-race` 需要 CGO 和受支持的 C 编译器；CI 在 Linux 上执行此项。

详细分析与后续优先级见 [项目审查报告](docs/review-and-roadmap.md)。

后续产品方向为天气感知的出行与生活规划 Agent。出行会话已实现，下一阶段覆盖天气备选、攻略 RAG、上下文预算及地图路线。具体阶段见 [出行 Agent 路线](docs/travel-agent-roadmap.md)；每个功能通过 GitHub Issue 和关联 PR 记录实现与验证，开发流程见 [AGENTS.md](AGENTS.md)。

## License

原项目声明使用 MIT；目前仓库没有独立 LICENSE 文件，正式分发前需补齐版权主体与许可证正文。
