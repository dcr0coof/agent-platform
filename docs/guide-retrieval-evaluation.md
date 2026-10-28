# 本地攻略检索质量基线

`cmd/guide-eval`直接调用生产`TravelGuide.Execute`，不复制检索算法、不接模型。固定自制语料位于`eval/travel-guide/corpus.md`，人工预期在`cases.json`。资料完全虚构，不能作为旅行营业信息使用。

在仓库根目录执行（Windows示例）：

```powershell
go build -o ./dist/guide-eval.exe ./cmd/guide-eval
./dist/guide-eval.exe -guide ./eval/travel-guide/corpus.md -suite ./eval/travel-guide/cases.json
$LASTEXITCODE
```

退出0表示全部预期吻合，1表示质量未达预期但已输出完整JSON报告，2表示输入/执行/输出错误，不产生有效质量报告。CI测试评测器行为，不能把单测通过当成语义检索质量通过。不要将质量退出1改成成功；保存标准输出时同时记录原退出码。

报告包含策略标签、资料与用例文件原始字节SHA256、逐题查询/预期/实际段落起始行、是否还有命中、exact和汇总。exact要求返回行序列及more_matches全部一致。无依据误召回只统计人工预期空数组的题目。空命中不是检索成功；缺少语义的查询可能被诚实地拒答但仍未满足用户需求。

资料hash必须与用例绑定值相等；资料变化则拒绝评分，需重新审查标注。用例限256 KiB、1–200题、唯一非空ID，预期必须为最多3条递增正整数的数组；还有命中时必须列出3条。未知字段、尾随文档、工具拒绝的查询也报错。JSON应由维护者审查；本命令不是面向不可信上传的评测服务。原始行号对应空行分段，不是语义标注或文本真实性认证。仓库对这两个输入及报告固定LF，避免Windows检出换行改变hash。

2026-10-27首次运行：**10/12 exact，无依据误召回0/3，退出1**，完整记录见[原始报告](../eval/travel-guide/report-2026-10-27.json)。本轮没有调优生产检索。

- `rain_paraphrase`：资料写“雨天”，查询“杭州 下雨”返回空，漏掉预期室内段落。
- `exclude_museum`：“杭州 不要博物馆”没有被理解为排除博物馆，返回空，漏掉图书馆/展馆/剧场。

该集合含英文大小写、AND匹配、多于三段、不同城市、无依据查询和两个意图探针。作者知道实现，不是盲测、真实用户样本或泛化准确率；也未覆盖模型选词、回答引用正确性、注入防护或资料时效。预期描述的是目标检索结果，故包含当前关键词契约外的语义需求。后续据此优化后它就成为调优回归集；应保留首次报告，并新增未参与调优的问题，不能改真值凑分。

关联[Issue #80](https://github.com/dcr0coof/agent-platform/issues/80)，依赖PR #79。只完成评测切片，不关闭上传RAG及完整评测路线。

## 2026-10-28：显式排除参数

guide-eval用例可增加可选exclude字符串并原样传给生产工具，报告保留该值；策略标签为literal-and-exclude-source-order-v2。单独的exclusion-cases.json验证人工指定参数后的过滤行为，不改cases.json的原始问句或预期。新增案例是契约回归，不能与原12条自然语言题直接合并计算“理解准确率”；原来的否定句仍未被自动理解。

复现新集合时将-suite改为./eval/travel-guide/exclusion-cases.json，-guide仍使用同一corpus.md。语料及旧用例hash不变，原始10月27日报告保留。
实际运行结果：原集合10/12，0/3无依据误召回，退出1，逐题结果与原报告完全一致；新增结构化集合7/7，0/2无依据误召回，退出0。结果分别见 [原集合复测](../eval/travel-guide/report-2026-10-28-original.json) 与 [排除契约报告](../eval/travel-guide/report-2026-10-28-exclusions.json)。这是两种不同输入任务，不能宣称原自然语言失败已修复。
