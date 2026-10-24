import { test, expect } from "@playwright/test";

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
  await page.getByRole("button", { name: "开启一段出行" }).click();
  await expect(card).toHaveCount(0);
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
  await expect(card).toContainText("本次关键词没有命中段落");
  await expect(card.locator(".guide-passage")).toHaveCount(0);
  await expect(card.locator("pre")).toHaveText(invalid);
  await expect(card.locator("b")).toHaveCount(0);
  await expect(card.locator(".guide-version")).toHaveCount(1);
});
