import { test, expect } from "@playwright/test";

test("weather evidence is paired per turn, escaped, restored and isolated", async ({ page }) => {
  await page.goto("/");
  const sessions = await (await page.request.get("/api/sessions")).json();
  const id = sessions[0].id;
  const session = await (await page.request.get(`/api/sessions/${id}`)).json();
  const call = (id: string, name: string) => ({ id, type: "function", function: { name, arguments: "{}" } });
  const evidence = "来源：QWeather\n获取时间：2026-09-19T02:00:00Z\n天气未知：日期未覆盖\n<script>window.injected=true</script>";
  session.messages = [
    { role: "user", content: "杭州天气？" },
    { role: "assistant", content: "", tool_calls: [call("w1", "weather"), call("c1", "calculator")] },
    { role: "tool", tool_call_id: "orphan", content: "孤立结果不可显示" },
    { role: "tool", tool_call_id: "c1", content: "非天气结果不可显示" },
    { role: "tool", tool_call_id: "w1", content: evidence },
    { role: "assistant", content: "请求日期天气未知。" },
    { role: "user", content: "新的问题，不查询天气" },
    { role: "tool", tool_call_id: "w1", content: "跨轮旧ID不可显示" },
    { role: "assistant", content: "普通回答。" },
  ];
  await page.route(`**/api/sessions/${id}`, route => route.fulfill({ json: session }));
  await page.reload();
  const cards = page.locator(".weather-evidence");
  await expect(cards).toHaveCount(1);
  await cards.locator("summary").click();
  await expect(cards.locator("pre")).toHaveText(evidence);
  await expect(cards.locator("script")).toHaveCount(0);
  expect(await page.evaluate(() => (window as unknown as { injected?: boolean }).injected)).toBeUndefined();
  await expect(page.locator(".message.assistant").last().locator("details")).toHaveCount(0);
  await page.setViewportSize({ width: 390, height: 844 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await page.reload();
  await expect(cards).toHaveCount(1);
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.getByRole("button", { name: "开启一段出行" }).click();
  await expect(cards).toHaveCount(0);
});
