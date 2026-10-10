import { test, expect, type Page } from "@playwright/test";

async function guarded(page: Page) {
  return page.evaluate(() => {
    const event = new Event("beforeunload", { cancelable: true });
    window.dispatchEvent(event);
    return event.defaultPrevented;
  });
}

test("leave guard tracks saved constraints and all page-local drafts", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByLabel("出行名称", { exact: true })).toBeEnabled();
  expect(await guarded(page)).toBe(false);
  await page.getByLabel("出行名称", { exact: true }).fill("原会话");
  expect(await guarded(page)).toBe(true);
  await page.getByRole("button", { name: "保存出行约束" }).click();
  await expect(page.getByText("出行约束已保存", { exact: true })).toBeVisible();
  expect(await guarded(page)).toBe(false);
  await page.getByLabel("出行消息").fill("留在原会话的草稿");
  expect(await guarded(page)).toBe(true);
  await page.getByRole("button", { name: "开启一段出行" }).click();
  await expect(page.locator(".trip-item")).toHaveCount(2);
  await expect(page.getByLabel("出行消息")).toHaveValue("");
  expect(await guarded(page)).toBe(true); // A different trip still has a draft.
  await page.getByRole("button", { name: /原会话/ }).click();
  await expect(page.getByLabel("出行消息")).toHaveValue("留在原会话的草稿");
  await page.getByLabel("出行消息").fill("");
  expect(await guarded(page)).toBe(false); // Ignore the active trip's stale map entry.
  await page.getByLabel("出行消息").fill("需要补充哪些约束？");
  await page.getByRole("button", { name: "发送消息" }).click();
  await expect(page.locator(".message.assistant")).toHaveCount(1);
  expect(await guarded(page)).toBe(false);
});

test("native reload confirmation can retain or discard an unsent draft", async ({ page }) => {
  await page.goto("/");
  await page.getByLabel("出行消息").fill("刷新前请保留");
  const dialog = page.waitForEvent("dialog");
  // Trigger browser navigation, not the automation protocol's reload command.
  await page.evaluate(() => { setTimeout(() => location.reload(), 0); });
  const warning = await dialog;
  expect(warning.type()).toBe("beforeunload");
  await warning.dismiss();
  await expect(page.getByLabel("出行消息")).toHaveValue("刷新前请保留");
  page.once("dialog", dialog => dialog.accept());
  await page.reload();
  await expect(page.getByLabel("出行消息")).toBeEnabled();
  await expect(page.getByLabel("出行消息")).toHaveValue("");
  expect(await guarded(page)).toBe(false);
});
