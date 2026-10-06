import { test, expect } from "@playwright/test";

test("confirmed count follows persisted constraints across edits and failed saves", async ({ page }) => {
  await page.goto("/");
  const count = page.locator(".completion > span");
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
  await expect(count).toContainText("已确认 0 / 6 项");
  await page.unroute("**/api/sessions/*");
  await page.getByRole("button", { name: "保存出行约束" }).click();
  await expect(count).toContainText("已确认 1 / 6 项");
  await page.getByLabel("目的地", { exact: true }).fill("");
  await expect(count).toContainText("已确认 1 / 6 项");
  await page.reload();
  await expect(count).toContainText("已确认 1 / 6 项");
  await expect(page.getByLabel("目的地", { exact: true })).toHaveValue("杭州");
  await page.getByLabel("目的地", { exact: true }).fill("");
  await page.getByRole("button", { name: "保存出行约束" }).click();
  await expect(count).toContainText("已确认 0 / 6 项");
});
