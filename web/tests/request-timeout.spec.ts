import { test, expect } from "@playwright/test";

test("a stalled trip load times out and leaves the original trip usable", async ({ page }) => {
  await page.clock.install();
  await page.goto("/");
  await page.getByLabel("出行名称", { exact: true }).fill("目标出行");
  await page.getByRole("button", { name: "保存出行约束" }).click();
  await expect(page.getByText("出行约束已保存", { exact: true })).toBeVisible();
  const target = (await (await page.request.get("/api/sessions")).json())[0].id;
  await page.getByRole("button", { name: "开启一段出行" }).click();
  await expect(page.locator(".trip-item")).toHaveCount(2);
  await page.getByLabel("出行消息").fill("保留未发送的内容");

  let release!: () => void;
  let reached!: () => void;
  const gate = new Promise<void>(resolve => { release = resolve; });
  const intercepted = new Promise<void>(resolve => { reached = resolve; });
  await page.route(`**/api/sessions/${target}`, async route => {
    reached();
    await gate;
    await route.continue();
  }, { times: 1 });
  try {
    await page.getByRole("button", { name: /目标出行/ }).click();
    await intercepted;
    await expect(page.getByLabel("出行消息")).toBeDisabled();
    await page.clock.fastForward(15_001);
    await expect(page.getByRole("alert")).toContainText("请求超时");
    await expect(page.getByRole("heading", { name: "新的出行", exact: true })).toBeVisible();
    await expect(page.getByLabel("出行消息")).toBeEnabled();
    await expect(page.getByLabel("出行消息")).toHaveValue("保留未发送的内容");
    await expect(page.getByRole("button", { name: "开启一段出行" })).toBeEnabled();
  } finally {
    release();
  }
  await page.getByRole("button", { name: "发送消息" }).click();
  await expect(page.locator(".message.user")).toContainText("保留未发送的内容");
  await expect(page.locator(".message.assistant")).toHaveCount(1);
  await page.getByRole("button", { name: /目标出行/ }).click();
  await expect(page.getByRole("heading", { name: "目标出行", exact: true })).toBeVisible();
});

test("retry after a lost start response reuses the accepted run", async ({ page }) => {
  await page.clock.install();
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "新的出行", exact: true })).toBeVisible();
  const requests: { request_id: string; message: string; revision: number }[] = [];
  const runIDs: string[] = [];
  let release!: () => void;
  let accepted!: () => void;
  const gate = new Promise<void>(resolve => { release = resolve; });
  const firstAccepted = new Promise<void>(resolve => { accepted = resolve; });
  await page.route("**/api/sessions/*/runs", async route => {
    requests.push(route.request().postDataJSON());
    const response = await route.fetch(); // Real server accepts the request.
    runIDs.push((await response.json()).id);
    if (requests.length === 1) {
      accepted();
      await gate; // Lose only the response, not the server-side operation.
    }
    await route.fulfill({ response });
  });
  try {
    await page.getByLabel("出行消息").fill("超时后继续同一个任务");
    await page.getByRole("button", { name: "发送消息" }).click();
    await firstAccepted;
    await page.clock.fastForward(15_001);
    await expect(page.getByRole("alert")).toContainText("请求超时");
    await expect(page.getByRole("alert")).toContainText("不代表服务端操作已取消");
    await expect(page.getByLabel("出行消息")).toHaveValue("超时后继续同一个任务");
    await expect(page.getByRole("button", { name: "发送消息" })).toBeEnabled();
    expect(requests).toHaveLength(1); // No automatic write retries.
    await page.getByRole("button", { name: "发送消息" }).click();
    await expect(page.locator(".message.assistant")).toHaveCount(1);
    await expect(page.locator(".message.user")).toHaveCount(1);
    expect(requests).toHaveLength(2);
    expect(requests[1]).toEqual(requests[0]);
    expect(runIDs).toEqual([runIDs[0], runIDs[0]]);
  } finally {
    release();
  }
  await page.reload();
  await expect(page.locator(".message.assistant")).toHaveCount(1);
  await expect(page.locator(".message.user")).toHaveCount(1);
});
