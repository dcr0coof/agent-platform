import { test, expect } from "@playwright/test";

test("save constraints, complete a run, restore on reload and isolate a second browser", async ({
  page,
  browser,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.goto("/");
  await expect(
    page.getByRole("heading", { name: "这次，想去哪里？" }),
  ).toBeVisible();
  await expect(page.getByText("本地演示", { exact: true })).toBeVisible();
  await page.getByLabel("出行名称", { exact: true }).fill("杭州轻松两日");
  await page.getByLabel("出发地", { exact: true }).fill("上海");
  await page.getByLabel("目的地", { exact: true }).fill("杭州");
  await page.getByLabel("开始日期").fill("2026-10-01");
  await page.getByLabel("结束日期").fill("2026-10-02");
  await page.getByLabel("出行人数", { exact: true }).fill("2");
  await page.getByLabel("总预算（元）", { exact: true }).fill("1500");
  await page.getByLabel("预算包含什么").selectOption("all");
  await page.getByLabel("偏好与特别要求").fill("少走路，不自驾");
  await page.getByRole("button", { name: "保存出行约束" }).click();
  await expect(page.getByText("出行约束已保存", { exact: true })).toBeVisible();
  await page.getByLabel("出行消息").fill("帮我核对条件");
  await page.getByRole("button", { name: "发送消息" }).click();
  await expect(page.locator(".message.assistant")).toContainText("上海 → 杭州");
  await expect(page.locator(".message.assistant")).toContainText(
    "少走路，不自驾",
  );
  await expect(page.locator(".timeline")).toContainText("回复已保存");
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "杭州轻松两日", exact: true }),
  ).toBeVisible();
  await expect(page.locator(".message.assistant")).toContainText("上海 → 杭州");
  await expect(page.getByLabel("偏好与特别要求")).toHaveValue("少走路，不自驾");
  await page.getByLabel("偏好与特别要求").fill("不去博物馆");
  await page.getByLabel("出行消息").fill("按新偏好重新核对");
  await page.getByRole("button", { name: "发送消息" }).click(); // Unsaved constraints are committed first.
  await expect(page.locator(".message.assistant").last()).toContainText(
    "不去博物馆",
  );
  await expect(page.locator(".message.assistant")).toHaveCount(2);
  const second = await browser.newContext();
  const other = await second.newPage();
  await other.goto("http://127.0.0.1:18081");
  await expect(
    other.getByRole("heading", { name: "新的出行", exact: true }),
  ).toBeVisible();
  await expect(other.locator(".message.assistant")).toHaveCount(0);
  await second.close();
  expect(errors).toEqual([]);
});

test("cancellation leaves no completed turn and another session remains independent", async ({
  page,
}) => {
  await page.goto("/");
  await page.getByLabel("出行消息").fill("这个任务要取消");
  await page.getByRole("button", { name: "发送消息" }).click();
  await page.getByRole("button", { name: "停止任务" }).click();
  await expect(page.locator(".run-notice")).toContainText("用户取消了任务");
  await expect(page.locator(".message.assistant")).toHaveCount(0);
  await page.reload();
  await expect(page.locator(".message.user")).toHaveCount(0);
  await page.getByRole("button", { name: "开启一段出行" }).click();
  await expect(page.locator(".trip-item")).toHaveCount(2);
  await page.getByLabel("出行消息").fill("还需要确认什么？");
  await page.getByRole("button", { name: "发送消息" }).click();
  await expect(page.locator(".message.assistant")).toContainText("出发地");
  await expect(page.locator(".message.assistant")).toContainText(
    "预算是否包含",
  );
});

test("mobile layout is usable without horizontal overflow", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/");
  await expect(
    page.getByRole("heading", { name: "这次，想去哪里？" }),
  ).toBeVisible();
  await page.getByRole("tab", { name: "出行约束" }).scrollIntoViewIfNeeded();
  await page.getByLabel("目的地", { exact: true }).fill("苏州");
  await page.getByRole("button", { name: "保存出行约束" }).click();
  await expect(page.getByText("出行约束已保存", { exact: true })).toBeVisible();
  const noOverflow = await page.evaluate(
    () => document.documentElement.scrollWidth <= window.innerWidth,
  );
  expect(noOverflow).toBeTruthy();
});
