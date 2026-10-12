import { test, expect } from "@playwright/test";

for (const outcome of ["cancelled", "failed"] as const) {
  test(`restore ${outcome} request without execution or silent draft loss`, async ({ page }) => {
    const requests: { request_id: string; message: string }[] = [];
    page.on("request", request => {
      if (request.method() === "POST" && /\/sessions\/[^/]+\/runs$/.test(request.url()))
        requests.push(request.postDataJSON());
    });
    // Demo has no provider failures. Exercise the same terminal API shape explicitly.
    if (outcome === "failed") await page.route("**/api/runs/*", async route => {
      const response = await route.fetch();
      const body = await response.json();
      if (body.status === "cancelled") { body.status = "failed"; body.error = "模型暂不可用（测试）"; }
      await route.fulfill({ response, json: body });
    });
    await page.goto("/");
    const input = page.getByLabel("出行消息");
    const restore = page.getByRole("button", { name: "恢复请求到输入框" });
    await expect(restore).toHaveCount(0);
    await input.fill("请继续规划杭州出行");
    await page.getByRole("button", { name: "发送消息" }).click();
    await page.getByRole("button", { name: "停止任务" }).click();
    await expect(restore).toBeEnabled();
    await expect(page.locator(".run-notice")).toContainText(outcome === "failed" ? "模型暂不可用" : "用户取消");
    await restore.click();
    await expect(input).toHaveValue("请继续规划杭州出行");
    await expect(input).toBeFocused();
    expect(requests).toHaveLength(1);
    await expect(page.locator(".message.assistant")).toHaveCount(0);
    await input.fill("已有草稿");
    page.once("dialog", dialog => dialog.dismiss());
    await restore.click();
    await expect(input).toHaveValue("已有草稿");
    page.once("dialog", dialog => dialog.accept());
    await restore.click();
    await expect(input).toHaveValue("请继续规划杭州出行");
    expect(requests).toHaveLength(1);
    await page.getByRole("button", { name: "发送消息" }).click();
    await expect(page.locator(".message.assistant")).toHaveCount(1);
    expect(requests).toHaveLength(2);
    expect(requests[1].request_id).not.toBe(requests[0].request_id);
    expect(requests[1].message).toBe(requests[0].message);
    await expect(restore).toHaveCount(0);
  });
}
