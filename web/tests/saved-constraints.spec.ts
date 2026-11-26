import { test, expect } from "@playwright/test";

test("confirmed count follows persisted constraints across edits and failed saves", async ({ page }) => {
  await page.goto("/");
  const count = page.locator(".completion > span");
  const missing = page.locator(".saved-missing");
  await expect(missing).toContainText("出发地、目的地、开始和结束日期、出行人数、预算金额、预算范围");
  await expect(count).toContainText("已确认 0 / 6 项");
  await page.getByLabel("目的地", { exact: true }).fill("杭州");
  await expect(count).toContainText("已确认 0 / 6 项");
  await expect(page.getByText("表单有未保存修改，确认计数将在保存成功后更新。")).toBeVisible();
  await page.route("**/api/sessions/*", async route => {
    if (route.request().method() === "PUT") await route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ error: "保存失败" }) });
    else await route.continue();
  });
  await page.getByRole("button", { name: "保存出行约束" }).click();
  await expect(page.getByRole("alert")).toContainText("保存失败");
  await expect(missing).toContainText("目的地");
  await expect(count).toContainText("已确认 0 / 6 项");
  await page.unroute("**/api/sessions/*");
  await page.getByRole("button", { name: "保存出行约束" }).click();
  await expect(count).toContainText("已确认 1 / 6 项");
  await expect(missing).not.toContainText("目的地");
  await page.getByLabel("目的地", { exact: true }).fill("");
  await expect(count).toContainText("已确认 1 / 6 项");
  page.once("dialog", dialog => dialog.accept()); // Explicitly discard the unsaved clear.
  await page.reload();
  await expect(count).toContainText("已确认 1 / 6 项");
  await expect(page.getByLabel("目的地", { exact: true })).toHaveValue("杭州");
  await expect(missing).not.toContainText("目的地");
  await page.getByLabel("目的地", { exact: true }).fill("");
  await page.getByRole("button", { name: "保存出行约束" }).click();
  await expect(count).toContainText("已确认 0 / 6 项");
  await expect(missing).toContainText("目的地");
});

test("missing names require both dates and reset on a new trip", async ({ page }) => {
  await page.goto("/");
  const missing = page.locator(".saved-missing");
  for (const [label, value] of [["出发地", "上海"], ["目的地", "杭州"], ["开始日期", "2028-02-28"], ["出行人数", "2"], ["总预算（元）", "1200"]])
    await page.getByLabel(label, { exact: true }).fill(value);
  await page.getByRole("combobox", { name: /预算包含什么/ }).selectOption("local");
  await page.getByRole("button", { name: "保存出行约束" }).click();
  await expect(missing).toHaveText("已保存条件待补充：开始和结束日期");
  await page.getByLabel("结束日期", { exact: true }).fill("2028-02-29");
  await expect(missing).toHaveText("已保存条件待补充：开始和结束日期");
  await page.getByRole("button", { name: "保存出行约束" }).click();
  await expect(missing).toHaveText("六项条件均已保存；仍需核对行程、预算和天气可行性。");
  await page.setViewportSize({width:390,height:844});
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.setViewportSize({width:1440,height:1000});
  await page.getByRole("button", {name:"开启一段出行"}).click();
  await expect(missing).toContainText("出发地、目的地、开始和结束日期、出行人数、预算金额、预算范围");
});
