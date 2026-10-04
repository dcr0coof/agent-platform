import { test, expect } from "@playwright/test";

test("new conversation keeps saved constraints without copying history or drafts", async ({ page }) => {
  await page.goto("/");
  await expect(page.locator(".trip-item")).toHaveCount(1);
  const fresh = page.getByRole("button", { name: "沿用约束开新对话", exact: true });
  await page.getByLabel("出行名称", { exact: true }).fill("杭州恢复计划");
  await page.getByLabel("目的地", { exact: true }).fill("杭州");
  await page.getByLabel("总预算（元）", { exact: true }).fill("1500");
  await page.getByLabel("预算包含什么").selectOption("local");
  await expect(fresh).toBeDisabled();
  await page.getByLabel("出行消息").fill("检查已保存条件");
  await page.getByRole("button", { name: "发送消息" }).click();
  await expect(fresh).toBeDisabled();
  await expect(page.locator(".message.assistant")).toHaveCount(1);
  await expect(fresh).toBeEnabled();
  const list = await (await page.request.get("/api/sessions")).json();
  const original = await (await page.request.get("/api/sessions/" + list[0].id)).json();
  await page.getByLabel("出行消息").fill("只属于原会话的草稿");
  await fresh.click();
  await expect(page.locator(".trip-item")).toHaveCount(2);
  await expect(page.getByRole("heading", { name: "杭州恢复计划 · 新对话", exact: true })).toBeVisible();
  await expect(page.getByLabel("出行消息")).toHaveValue("");
  await expect(page.locator(".message")).toHaveCount(0);
  await expect(page.getByText("已沿用保存的约束开启新对话；原会话和草稿保留，旧聊天未带入。", { exact: true })).toBeVisible();
  const after = await (await page.request.get("/api/sessions")).json();
  const copy = await (await page.request.get("/api/sessions/" + after.find((s: { id: string }) => s.id !== original.id).id)).json();
  expect(copy.constraints).toEqual(original.constraints);
  expect(copy.messages || []).toEqual([]);
  expect(copy.active_run).toBeUndefined();
  expect(await (await page.request.get("/api/sessions/" + original.id)).json()).toEqual(original);
  await page.getByRole("navigation", { name: "出行会话" }).getByRole("button").filter({ hasText: "杭州恢复计划" }).filter({ hasNotText: "新对话" }).click();
  await expect(page.getByLabel("出行消息")).toHaveValue("只属于原会话的草稿");
  await expect(page.locator(".message.assistant")).toHaveCount(1);
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(fresh).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
});

test("failed creation preserves original selection and allows retry", async ({ page }) => {
  await page.goto("/");
  await expect(page.locator(".trip-item")).toHaveCount(1);
  await page.getByLabel("出行消息").fill("原草稿");
  await page.route("**/api/sessions", async route => {
    if (route.request().method() === "POST") {
      await route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ error: "新会话创建失败" }) });
    } else await route.continue();
  });
  const fresh = page.getByRole("button", { name: "沿用约束开新对话", exact: true });
  await fresh.click();
  await expect(page.getByRole("alert")).toContainText("新会话创建失败");
  await expect(page.locator(".trip-item")).toHaveCount(1);
  await expect(page.getByLabel("出行消息")).toHaveValue("原草稿");
  await expect(fresh).toBeEnabled();
  await page.unroute("**/api/sessions");
  await fresh.click();
  await expect(page.locator(".trip-item")).toHaveCount(2);
  await expect(page.getByLabel("出行消息")).toHaveValue("");
});
