# 行迹 Windows 演示候选包

仅适用于 Windows x64。解压整个 ZIP 到可写目录，双击 `start-demo.cmd`，打开 http://127.0.0.1:8080；Ctrl+C 停止。无需安装 Go、Node.js，也不需要 API 密钥。8080 已被占用时关闭本程序后在包目录使用 `server.exe -demo -listen 127.0.0.1:8081` 并访问对应端口。

可以创建出行、保存约束、连续对话、取消任务、刷新恢复已保存数据及导出记录。数据在包内 `data/trips.db`，浏览器 Cookie 用来隔离会话；清除 Cookie 会失去界面访问原会话的入口。升级前关闭程序并备份 data，保留旧包；请勿向别人分发含个人数据库的运行目录。同一个数据库只运行一个服务实例。

**这是明确标注的本地规则演示，不会调用模型或天气服务，不提供真实天气或旅行建议。** 天气证据显示通过模拟测试覆盖，不代表真实服务联调成功。攻略 RAG、token 预算与摘要、地图和行程生成未完成。真实模式需要另外配置，`configs/config.example.yaml` 仅供参考，不属于本包已验收范围。

下载后将 `SHA256SUMS.txt` 中对应 ZIP 的值与 PowerShell `Get-FileHash .\agent-platform-v0.1.0-demo.1-windows-amd64.zip -Algorithm SHA256` 比较（文件名按实际版本替换）。`build-info.json` 标明版本、源提交及构建时是否存在未提交修改；带未提交修改的包只能用于本地验证。

此打包流程生成候选产物，不表示已经发布 Release。正式预发布必须从正常评审合入 master、CI 通过的提交构建，并再次验证解压后的核心流程。仓库当前缺少独立许可证正文，公开分发前还需补齐版权与许可证说明。问题追踪：https://github.com/dcr0coof/agent-platform/issues 。
