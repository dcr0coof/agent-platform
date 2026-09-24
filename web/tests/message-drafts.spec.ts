import { test, expect } from "@playwright/test";

test("unsent drafts follow their trip through creation, switching and sending", async ({ page }) => {
  await page.goto("/");
  await page.getByLabel("出行名称", { exact: true }).fill("杭州出行");
  await page.getByRole("button", { name: "保存出行约束" }).click();
  await expect(page.getByText("出行约束已保存", { exact: true })).toBeVisible();
  const input = page.getByLabel("出行消息");
  await input.fill("杭州：需要雨天备选");
  await expect(page.getByText("未发送消息按出行暂存于本页，刷新或关闭页面后清除。", { exact: true })).toBeVisible();

  await page.getByRole("button", { name: "开启一段出行" }).click();
  await expect(page.locator(".trip-item")).toHaveCount(2);
  await expect(input).toHaveValue("");
  await input.fill("苏州：希望少走路");
  await page.getByLabel("出行名称", { exact: true }).fill("苏州出行");
  await page.getByRole("button", { name: "保存出行约束" }).click();
  await expect(page.getByText("出行约束已保存", { exact: true })).toBeVisible();
  await expect(input).toHaveValue("苏州：希望少走路");

  await page.getByRole("button", { name: /杭州出行/ }).click();
  await expect(input).toHaveValue("杭州：需要雨天备选");
  await page.getByRole("button", { name: /苏州出行/ }).click();
  await expect(input).toHaveValue("苏州：希望少走路");
  await page.getByRole("button", { name: "发送消息" }).click();
  await expect(page.locator(".message.assistant")).toHaveCount(1);
  await expect(input).toHaveValue("");
  await page.getByRole("button", { name: /杭州出行/ }).click();
  await expect(input).toHaveValue("杭州：需要雨天备选");
  await page.getByRole("button", { name: /苏州出行/ }).click();
  await expect(input).toHaveValue("");
  await expect(page.locator(".message.user")).toContainText("苏州：希望少走路");
  await page.getByRole("button", { name: /杭州出行/ }).click();
  await expect(input).toHaveValue("杭州：需要雨天备选");
  await page.reload();
  await expect(page.getByRole("heading", { name: "杭州出行", exact: true })).toBeVisible();
  await expect(input).toHaveValue("");
});

test("failed trip selection and same-trip reload preserve the active draft", async ({ page }) => {
  await page.goto("/");
  await page.getByLabel("出行名称", { exact: true }).fill("目标出行");
  await page.getByRole("button", { name: "保存出行约束" }).click();
  await expect(page.getByText("出行约束已保存", { exact: true })).toBeVisible();
  const list = await (await page.request.get("/api/sessions")).json();
  await page.getByLabel("出行消息").fill("目标草稿");
  await page.getByRole("button", { name: "开启一段出行" }).click();
  await expect(page.locator(".trip-item")).toHaveCount(2);
  await page.getByLabel("出行消息").fill("当前草稿");
  await page.route(`**/api/sessions/${list[0].id}`, route => route.fulfill({
    status: 503, contentType: "application/json", body: JSON.stringify({ error: "加载目标失败" }),
  }));
  await page.getByRole("button", { name: /目标出行/ }).click();
  await expect(page.getByRole("alert")).toContainText("加载目标失败");
  await expect(page.getByLabel("出行消息")).toHaveValue("当前草稿");
  await page.getByRole("button", { name: "刷新", exact: true }).click();
  await expect(page.getByRole("alert")).toHaveCount(0);
  await expect(page.getByLabel("出行消息")).toHaveValue("当前草稿");
  await page.unroute(`**/api/sessions/${list[0].id}`);
  await page.getByRole("button", { name: /目标出行/ }).click();
  await expect(page.getByLabel("出行消息")).toHaveValue("目标草稿");
});
