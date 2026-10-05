import { test, expect } from "@playwright/test";

for (const width of [1440, 390]) {
  test(`editorial workspace loads local artwork and remains usable at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 1000 });
    await page.emulateMedia({ reducedMotion: "reduce" });
    const externalRequests: string[] = [];
    page.on("request", request => {
      if (new URL(request.url()).hostname !== "127.0.0.1") externalRequests.push(request.url());
    });
    await page.goto("/");
    await expect(page.getByRole("heading", { name: "这次，想去哪里？" })).toBeVisible();
    await expect.poll(() => page.locator("img").evaluateAll(images =>
      images.every(image => image.complete && image.naturalWidth > 0),
    )).toBeTruthy();
    expect(externalRequests).toEqual([]);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy();
    await page.screenshot({ path: testInfo.outputPath(`travel-${width}.png`), fullPage: true });
    const prompt = page.getByRole("button", { name: /检查我的出行条件/ });
    await prompt.focus();
    await page.keyboard.press("Enter");
    await expect(page.getByLabel("出行消息")).toHaveValue("帮我检查这次出行还缺少哪些信息。");
    if (width === 390) {
      await page.getByRole("button", { name: "打开会话列表" }).click();
      await page.getByRole("button", { name: "开启一段出行" }).click();
      await expect(page.locator(".trip-item")).toHaveCount(2);
      await expect(page.locator(".sidebar")).not.toBeVisible();
    }
  });
}
