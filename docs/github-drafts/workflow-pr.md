# docs: 明确出行 Agent 路线和逐步发布工作流

## 问题与结果

项目原有规划以天气工具和通用知识问答为主，缺少把天气、出行、RAG、上下文和前端连成用户任务的路线，也没有持续的 Issue/PR 工作约定。本 PR 明确天气感知的出行与生活规划场景，按可验收功能切片定义依赖，并增加开发约定及 Issue/PR 模板。

## 关联 Issue

Closes #2

已发布为 [PR #11](https://github.com/dcr0coof/agent-platform/pull/11)，依赖 [PR #10](https://github.com/dcr0coof/agent-platform/pull/10)。当前以基线分支为比较目标，基线合并后调整为 master。

## 变更与验证

- 新增仓库工作约定、领域术语及技能所需的追踪规则。
- 新增出行 Agent 路线与完整 Issue 草稿，区分已完成能力和计划。
- 添加功能、缺陷和 PR 模板；明确每个功能 Issue → 分支 → 验证 → 推送 → PR 的流程。
- 文档与 Git diff 检查；本 PR 不修改运行时代码。

## 限制

9 个 Issues 和 2 个 PR 已发布。对应链接与依赖见发布记录；功能 Issue 的创建不代表功能已实现，PR 创建不代表已经合并。
