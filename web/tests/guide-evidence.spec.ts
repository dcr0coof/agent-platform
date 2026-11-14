import { test, expect } from "@playwright/test";
import { readFile } from "node:fs/promises";

async function downloadText(page: import("@playwright/test").Page) {
  const pending = page.waitForEvent("download");
  await page.getByRole("button", { name: "导出出行记录", exact: true }).click();
  const download = await pending;
  expect(await download.failure()).toBeNull();
  expect(download.suggestedFilename()).toMatch(/\.txt$/);
  return readFile((await download.path())!, "utf8");
}

test("guide citations are paired, escaped, restored and separate from weather", async ({ page }) => {
  await page.goto("/");
  await expect(page.locator(".trip-item")).toHaveCount(1);
  const sessions = await (await page.request.get("/api/sessions")).json();
  const id = sessions[0].id;
  const session = await (await page.request.get(`/api/sessions/${id}`)).json();
  const call = (id: string, name = "search_travel_guide") => ({ id, type: "function", function: { name, arguments: "{}" } });
  const evidence = { source: "<img src=x onerror=window.injected=true>.md", sha256: "a".repeat(64), notice: "静态资料，非实时信息", matches: [{ start_line: 3, end_line: 4, text: "杭州雨天\n<script>window.injected=true</script>" }], more_matches: true };
  session.messages = [
    { role: "user", content: "雨天攻略与天气" },
    { role: "assistant", content: "", tool_calls: [call("g1"), call("w1", "weather"), call("c1", "calculator"), call("missing")] },
    { role: "tool", tool_call_id: "orphan", content: "孤立结果" },
    { role: "tool", tool_call_id: "c1", content: "计算结果" },
    { role: "tool", tool_call_id: "w1", content: "天气专属记录" },
    { role: "tool", tool_call_id: "g1", content: JSON.stringify(evidence) },
    { role: "tool", tool_call_id: "g1", content: "重复结果" },
    { role: "assistant", content: "参考静态资料，开放情况另查。" },
    { role: "user", content: "普通问题" },
    { role: "tool", tool_call_id: "missing", content: "跨轮结果" },
    { role: "assistant", content: "普通回答。" },
  ];
  await page.route(`**/api/sessions/${id}`, route => route.fulfill({ json: session }));
  await page.reload();
  const card = page.locator(".guide-evidence");
  await expect(card).toHaveCount(1);
  await card.locator("summary").focus();
  await page.keyboard.press("Enter");
  await expect(card).toHaveAttribute("open", "");
  await expect(card).toContainText(evidence.source);
  await expect(card).toContainText(evidence.sha256);
  await expect(card).toContainText("原文第 3–4 行");
  await expect(card.locator("pre")).toHaveText(evidence.matches[0].text);
  await expect(card).toContainText("还有其他命中段落未展示");
  for (const text of ["天气专属记录", "计算结果", "重复结果", "跨轮结果", "孤立结果"]) await expect(card).not.toContainText(text);
  await expect(page.locator(".weather-evidence:not(.guide-evidence) pre")).toHaveText("天气专属记录");
  await expect(card.locator("script, img, a")).toHaveCount(0);
  expect(await page.evaluate(() => (window as any).injected)).toBeUndefined();
  await expect(page.locator(".message.assistant").last().locator("details")).toHaveCount(0);
  await page.setViewportSize({ width: 390, height: 844 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.reload();
  await expect(card).toHaveCount(1);
  await page.setViewportSize({ width: 1440, height: 1000 });
  const record = await downloadText(page);
  expect(record).toContain(JSON.stringify(evidence));
  expect(record).toContain("天气专属记录");
  expect(record).toContain("不代表实时事实或回答已正确引用");
  expect(record.split(JSON.stringify(evidence))).toHaveLength(2);
  for (const text of ["计算结果", "重复结果", "跨轮结果", "孤立结果"]) expect(record).not.toContain(text);
  await page.getByRole("button", { name: "开启一段出行" }).click();
  await expect(card).toHaveCount(0);
  expect(await downloadText(page)).not.toContain(evidence.sha256);
});

test("no hit and invalid guide results never invent citations", async ({ page }) => {
  await page.goto("/");
  await expect(page.locator(".trip-item")).toHaveCount(1);
  const sessions = await (await page.request.get("/api/sessions")).json();
  const id = sessions[0].id;
  const session = await (await page.request.get(`/api/sessions/${id}`)).json();
  const valid = { source: "guide.md", sha256: "b".repeat(64), notice: "静态资料", matches: [], more_matches: false };
  const invalid = ["工具错误：<b>读取失败</b>", "null", JSON.stringify({ ...valid, sha256: "bad" }), JSON.stringify({ ...valid, matches: [{ start_line: 0, end_line: 2, text: "invalid lines" }] }), JSON.stringify({ ...valid, matches: [{ start_line: 3, end_line: 2, text: "reversed" }] })];
  session.messages = [
    { role: "user", content: "无答案及异常" },
    { role: "assistant", content: "", tool_calls: [JSON.stringify(valid), ...invalid].map((_, i) => ({ id: `g${i}`, type: "function", function: { name: "search_travel_guide", arguments: "{}" } })) },
    ...[JSON.stringify(valid), ...invalid].map((content, i) => ({ role: "tool", tool_call_id: `g${i}`, content })),
    { role: "assistant", content: "缺少攻略依据。" },
  ];
  await page.route(`**/api/sessions/${id}`, route => route.fulfill({ json: session }));
  await page.reload();
  const card = page.locator(".guide-evidence");
  await card.locator("summary").click();
  await expect(card).toContainText("本次查询条件下没有可用段落");
  await expect(card.locator(".guide-passage")).toHaveCount(0);
  await expect(card.locator("pre")).toHaveText(invalid);
  await expect(card.locator("b")).toHaveCount(0);
  await expect(card.locator(".guide-version")).toHaveCount(1);
  const record = await downloadText(page);
  for (const raw of [JSON.stringify(valid), ...invalid]) expect(record).toContain(raw);
});

test("fully filtered evidence explains the constraint and survives export", async ({ page }) => {
  await page.goto("/");
  await expect(page.locator(".trip-item")).toHaveCount(1);
  const sessions = await (await page.request.get("/api/sessions")).json();
  const id = sessions[0].id;
  const session = await (await page.request.get(`/api/sessions/${id}`)).json();
  const evidence = {
    source: "guide.md", sha256: "c".repeat(64), matches: [], more_matches: false,
    query_matches: 5, excluded_matches: 5, query_terms: ["杭州"], excluded_terms: ["博物馆"],
    notice: "关键词命中 5 段，排除 5 段，剩余 0 段。命中段落均被排除条件过滤；未自动放宽条件，请补充其他资料或由用户明确调整条件。",
  };
  session.messages = [
    { role: "user", content: "杭州，排除博物馆" },
    { role: "assistant", content: "", tool_calls: [{ id: "filtered", type: "function", function: { name: "search_travel_guide", arguments: '{"query":"杭州","exclude":"博物馆"}' } }] },
    { role: "tool", tool_call_id: "filtered", content: JSON.stringify(evidence) },
    { role: "assistant", content: "需要其他资料。" },
  ];
  await page.route(`**/api/sessions/${id}`, route => route.fulfill({ json: session }));
  await page.reload();
  const card = page.locator(".guide-evidence");
  await card.locator("summary").click();
  await expect(card).toContainText(evidence.notice);
  await expect(card.locator(".guide-filters")).toContainText('实际检索词（全部匹配）：["杭州"]');
  await expect(card.locator(".guide-filters")).toContainText('实际排除词（任一命中即排除整段）：["博物馆"]');
  await expect(card).toContainText("本次查询条件下没有可用段落");
  await expect(card).not.toContainText("关键词没有命中");
  await expect(card.locator(".guide-passage")).toHaveCount(0);
  expect(await downloadText(page)).toContain(JSON.stringify(evidence));
  await expect(card.locator(".guide-coverage")).toContainText("筛选后共 0 段，本次展示 0 段，未展示 0 段");
});

test("coverage distinguishes truncation and rejects contradictory statistics", async ({ page }) => {
  await page.goto("/");
  await expect(page.locator(".trip-item")).toHaveCount(1);
  const [{ id }] = await (await page.request.get("/api/sessions")).json();
  const session = await (await page.request.get(`/api/sessions/${id}`)).json();
  const base = { source: "guide.md", sha256: "e".repeat(64), notice: "静态资料", matches: [1, 3, 5].map(n => ({ start_line: n, end_line: n, text: "杭州" })), more_matches: true };
  const stats = [{ query_matches: 8, excluded_matches: 2 }, {}, { query_matches: 2, excluded_matches: 0 }, { query_matches: 6, excluded_matches: -1 }, { query_matches: 3, excluded_matches: 0 }, { query_matches: 4.5, excluded_matches: 0 }, { query_matches: 1, excluded_matches: 2 }];
  session.messages = [
    { role: "user", content: "核对段落覆盖范围" },
    { role: "assistant", content: "", tool_calls: stats.map((_, i) => ({ id: `c${i}`, type: "function", function: { name: "search_travel_guide", arguments: "{}" } })) },
    ...stats.map((s, i) => ({ role: "tool", tool_call_id: `c${i}`, content: JSON.stringify({ ...base, ...s }) })),
    { role: "assistant", content: "资料" },
  ];
  await page.route(`**/api/sessions/${id}`, route => route.fulfill({ json: session }));
  await page.reload();
  const card = page.locator(".guide-evidence");
  await card.locator("summary").click();
  const coverage = card.locator(".guide-coverage");
  await expect(coverage.first()).toContainText("筛选后共 6 段，本次展示 3 段，未展示 3 段");
  for (let i = 1; i < stats.length; i++) await expect(coverage.nth(i)).toContainText("段落覆盖数量未知");
  await expect(card.locator(".guide-passage")).toHaveCount(stats.length * 3);
});

test("effective filter metadata is escaped and legacy or invalid metadata stays unknown", async ({ page }) => {
  await page.goto("/");
  await expect(page.locator(".trip-item")).toHaveCount(1);
  const [{ id }] = await (await page.request.get("/api/sessions")).json();
  const session = await (await page.request.get(`/api/sessions/${id}`)).json();
  const base = { source: "guide.md", sha256: "d".repeat(64), notice: "静态资料", matches: [], more_matches: false };
  const variants = [
    { query_terms: ["<img src=x onerror=window.injected=true>", "杭州"], excluded_terms: [] },
    {},
    { query_terms: ["杭州"] },
    { query_terms: [], excluded_terms: [] },
    { query_terms: ["杭州"], excluded_terms: null },
    { query_terms: ["杭州"], excluded_terms: [42] },
    { query_terms: ["杭州"], excluded_terms: ["　"] },
    { query_terms: Array(7).fill("杭州"), excluded_terms: [] },
  ];
  session.messages = [
    { role: "user", content: "核对筛选条件" },
    { role: "assistant", content: "", tool_calls: variants.map((_, i) => ({ id: `f${i}`, type: "function", function: { name: "search_travel_guide", arguments: '{"query":"不能从参数推断"}' } })) },
    ...variants.map((v, i) => ({ role: "tool", tool_call_id: `f${i}`, content: JSON.stringify({ ...base, ...v }) })),
    { role: "assistant", content: "核对完成" },
  ];
  await page.route(`**/api/sessions/${id}`, route => route.fulfill({ json: session }));
  await page.reload();
  const card = page.locator(".guide-evidence");
  await card.locator("summary").click();
  const filters = card.locator(".guide-filters");
  await expect(filters).toHaveCount(variants.length);
  await expect(filters.first()).toContainText(JSON.stringify(variants[0].query_terms));
  await expect(filters.first()).toContainText("未启用排除");
  for (let i = 1; i < variants.length; i++) await expect(filters.nth(i)).toContainText("实际筛选词未知");
  await expect(card.locator(".guide-version")).toHaveCount(variants.length);
  await expect(card).not.toContainText("不能从参数推断");
  await expect(card.locator("img, script")).toHaveCount(0);
  expect(await page.evaluate(() => (window as any).injected)).toBeUndefined();
  expect(await downloadText(page)).toContain(JSON.stringify({ ...base, ...variants[0] }));
});
