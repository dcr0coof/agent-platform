# 行迹：本机出行工作台

对应 [Issue #3](https://github.com/dcr0coof/agent-platform/issues/3)。本切片提供出行会话基础，并未实现自动生成完整行程、攻略检索或地图。

## 启动

需要 Go 1.22.12+ 和 Node.js 22.12+。在仓库根目录运行：

```bash
npm --prefix web ci
npm --prefix web run build
go run ./cmd/server -demo
```

访问 <http://127.0.0.1:8080>。创建出行，在约束面板填入目的地、起止日期、出发地、人数、总预算及范围、偏好。发送前会保存尚未保存的表单。缺失条件会提示补充，运行期间不能编辑当前约束。执行详情显示真实事件；取消后该轮不会进入成功对话。

同一标签页刷新后恢复上次打开的出行，即使它不是最近更新的会话。选择仅用 `sessionStorage` 保存会话 ID，不保存对话或凭据；不同标签页后续选择互不影响。恢复前核对服务器返回的可访问列表；ID 失效或浏览器禁用存储时回退到最近更新的会话，列表为空则创建新会话。关闭标签页后不保证保留选择，浏览器复制标签页可能继承初始选择。

演示是规则回复，不调用外部服务。接入模型时移除 `-demo`，沿用 README 的环境变量配置：

```powershell
$env:AGENT_LLM_API_KEY = "your-api-key"
go run ./cmd/server
```

天气工具仅在配置天气密钥和专属 API Host 后启用。当前模型回复为整段返回；SSE 流式更新的是执行状态，不是 token。天气证据卡和天气驱动的替代活动留在 Issue #4。

可选参数：`-listen 127.0.0.1:8080`、`-db data/trips.db`、`-web-dir web/dist`、`-timeout 2m`、`-config configs/config.yaml`。演示与真实模式可使用不同数据库文件。每个文件只能供一个服务实例使用。停止服务后可复制数据库备份；运行中的 WAL 数据库不应只复制主文件。

开发热更新：一个终端运行 `go run ./cmd/server -demo -dev-origin http://127.0.0.1:5173`，另一个运行 `npm --prefix web run dev`，浏览器使用 <http://127.0.0.1:5173>。Vite 将 `/api` 转发到 Go 的 8080 端口。

## 数据与 HTTP API

首先 `GET /api/bootstrap`，浏览器自动接收本机工作区 Cookie。其他请求必须携带它；API 不开放跨站 CORS。JSON 请求使用 `Content-Type: application/json`，最大 32 KiB，不接受未知字段。

| 方法与路径 | 行为 |
| --- | --- |
| `GET /api/bootstrap` | 创建浏览器标识，返回当前演示模式和能力 |
| `GET /api/sessions` | 当前浏览器的会话列表 |
| `POST /api/sessions` | 创建会话：`title`、`constraints` |
| `GET /api/sessions/{id}` | 读取条件、完整成功历史、版本号及活动任务 |
| `PUT /api/sessions/{id}` | 更新 `title`、`constraints`、`revision`；过期版本或运行中返回 409 |
| `POST /api/sessions/{id}/runs` | 提交 `message`、`request_id`、`revision`，返回 202 与任务 |
| `GET /api/runs/{id}` | 任务状态和有序事件 |
| `POST /api/runs/{id}/cancel` | 取消任务；对已完成任务返回已有终态 |
| `GET /api/runs/{id}/events` | SSE 订阅，可用 `Last-Event-ID` 或 `?after=N` 续读 |

会话列表仅从数据库选取导航需要的元数据，`messages` 为 `null`，不加载完整聊天 JSON；完整历史仍由会话详情接口读取。历史格式损坏时列表仍可显示该会话，详情会明确报错，不会自动清空或修复历史。列表尚未分页，会话数量本身很大时仍需另行优化。

约束对象示例：

```json
{
  "origin": "上海",
  "destination": "杭州",
  "start_date": "2026-09-12",
  "end_date": "2026-09-13",
  "travelers": 2,
  "budget": 1500,
  "budget_scope": "all",
  "preferences": "不自驾，尽量少走路"
}
```

预算当前为人民币总额。`all` 包含往返交通与住宿，`local` 只含当地活动；空值表示尚未确认。人数和预算为 0 表示待补充；表单允许不完整条件，但禁止非法日期、结束早于开始、负数等值。

每次用户发送生成独立 `request_id`（8～80 位字母、数字、下划线或连字符）。同一会话的相同幂等键及消息重试会返回原任务；改变消息却复用键返回冲突。消息最长 16000 字节。每会话一轮、服务最多四轮同时执行，容量不足返回 429。

SSE `data` 是 `{seq,type,detail,created_at}`，`id` 等于 `seq`。终态事件为 `run.completed`、`run.failed` 或 `run.cancelled`。客户端收到终态后关闭连接并重新获取会话。断开订阅不会取消任务；显式取消或整体超时才停止执行。失败、取消和重启遗留任务不会写入不完整聊天；重连不会重新发起工具。

## 隔离与当前限制

所有会话、任务、取消和事件接口都按 Cookie 归属检查；未知或其他浏览器的资源返回 404。Cookie 是本机访问凭据，不是账号系统；清理 Cookie 或使用新浏览器会创建新的空间，原数据不会自动删除或迁移。数据库未加密，公开部署、多实例运行、用户管理和数据删除界面尚未提供。

已确认约束每轮重新进入系统上下文，完整成功历史持久保存，模型端继续按完整回合裁剪。当前没有 RAG、摘要或 token 上限；数据库持久化不能替代上下文预算。架构取舍见 [ADR 0001](adr/0001-local-trip-workspace.md)。

## 验证

列表查询的小规模分配实验（2026-09-11，Windows amd64、Go 1.22.12，单会话含 1 MiB 文本历史，每个版本测量 5 次）：修改前 `4,222,864 B/op`，修改后 `2,584 B/op`。这是 Go 堆分配量，不是进程常驻内存或物理磁盘读取量；不能据此推断生产环境延迟。数据写入在计时前完成，基准不调用模型。复现命令：

```bash
go test -p=2 ./internal/trip -run '^$' -bench '^BenchmarkListLargeHistory$' -benchtime=5x -count=1 -benchmem
```

在仓库根目录执行：

```bash
go test ./...
go vet ./...
go build ./...
npm --prefix web run build
```

浏览器测试会在 18081 端口启动独立演示服务，使用隔离的临时数据库，无需真实密钥：

```bash
cd web
npx playwright install chromium
npm run test:e2e
```

若本机已有 Chrome，可在 PowerShell 设置 `$env:PLAYWRIGHT_CHANNEL = "chrome"` 后直接运行浏览器测试。Linux CI 安装 Chromium 和系统依赖；Go CI 在 Windows / Linux 运行，Linux 额外执行 `go test -race ./...`。未运行实际付费模型请求时，只能声明模拟客户端协议与界面路径已验证。
