import { test, expect } from "@playwright/test";

for (const outcome of ["success", "failure", "switch"] as const) {
  test(`terminal refresh keeps edits locked and handles ${outcome}`, async ({ page }) => {
    await page.goto("/");
    await expect(page.getByLabel("出行名称", { exact: true })).toBeEnabled();
    const sessions = await (await page.request.get("/api/sessions")).json();
    const sid = sessions[0].id;
    let release!: () => void;
    let reached!: () => void;
    const gate = new Promise<void>(resolve => { release = resolve; });
    const intercepted = new Promise<void>(resolve => { reached = resolve; });
    await page.route(`**/api/sessions/${sid}`, async route => {
      const response = await route.fetch();
      reached();
      await gate;
      if (outcome === "failure") await route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ error: "结果刷新失败" }) });
      else await route.fulfill({ response });
    });
    await page.getByLabel("出行消息").fill("检查结束后的交互");
    await page.getByRole("button", { name: "发送消息" }).click();
    await intercepted;
    const responseDelivered = page.waitForResponse(response => response.url().endsWith(`/api/sessions/${sid}`));
    try {
      await page.getByRole("tab", { name: "出行约束" }).click();
      await expect(page.getByLabel("出行名称", { exact: true })).toBeDisabled();
      await expect(page.getByLabel("出行消息")).toBeDisabled();
      await expect(page.locator(".save-button")).toBeDisabled();
      await expect(page.getByRole("button", { name: "发送消息" })).toBeDisabled();
      await expect(page.getByRole("button", { name: "导出出行记录" })).toBeDisabled();
      await expect(page.getByRole("button", { name: "沿用约束开新对话" })).toBeDisabled();
      if (outcome === "switch") {
        await page.getByRole("button", { name: "开启一段出行" }).click();
        await expect(page.locator(".trip-item")).toHaveCount(2);
        await expect(page.getByLabel("出行名称", { exact: true })).toBeEnabled();
        await page.getByLabel("出行名称", { exact: true }).fill("新会话草稿");
      }
    } finally {
      release();
      await (await responseDelivered).finished();
    }
    await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
    await expect(page.getByLabel("出行名称", { exact: true })).toBeEnabled();
    if (outcome === "failure") await expect(page.getByRole("alert")).toContainText("结果刷新失败");
    else if (outcome === "switch") {
      await expect(page.getByLabel("出行名称", { exact: true })).toHaveValue("新会话草稿");
      await expect(page.locator(".message.assistant")).toHaveCount(0);
    } else await expect(page.locator(".message.assistant")).toHaveCount(1);
  });
}
