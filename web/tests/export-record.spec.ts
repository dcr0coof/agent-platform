import { test, expect, type Page } from "@playwright/test";
import { readFile } from "node:fs/promises";

async function downloadRecord(page: Page) {
  const pending = page.waitForEvent("download");
  await page.getByRole("button", { name: "导出出行记录", exact: true }).click();
  const download = await pending;
  expect(await download.failure()).toBeNull();
  return { filename: download.suggestedFilename(), text: await readFile((await download.path())!, "utf8") };
}

test("download uses saved constraints and completed turns, excluding drafts and other trips", async ({ page }) => {
  await page.goto("/");
  await expect(page.locator(".trip-item")).toHaveCount(1);
  const blank = await downloadRecord(page);
  expect(blank.text).toContain("暂无已完成对话");
  expect(blank.text).toContain("总预算（人民币元）：待确认");
  await page.getByLabel("出行名称", { exact: true }).fill('杭州/两日:记录');
  await page.getByLabel("目的地", { exact: true }).fill("杭州");
  await page.getByLabel("总预算（元）", { exact: true }).fill("1500");
  await page.getByLabel("预算包含什么").selectOption("local");
  await page.getByLabel("出行消息").fill("核对我已保存的杭州计划");
  await page.getByRole("button", { name: "发送消息" }).click();
  await expect(page.getByRole("button", { name: "导出出行记录" })).toBeDisabled();
  await expect(page.locator(".message.assistant")).toHaveCount(1);
  await page.getByRole("tab", { name: "出行约束" }).click();
  await page.getByLabel("目的地", { exact: true }).fill("未保存的北京");
  await page.getByLabel("出行消息").fill("不能导出的草稿");
  const record = await downloadRecord(page);
  expect(record.filename).toBe("行迹-杭州_两日_记录.txt");
  for (const text of ["目的地：杭州", "总预算（人民币元）：1500", "仅当地活动，不含交通住宿", "核对我已保存的杭州计划", "本地演示", "历史天气依据不代表当前天气", "导出时间："])
    expect(record.text).toContain(text);
  expect(record.text).not.toContain("未保存的北京");
  expect(record.text).not.toContain("不能导出的草稿");
  await page.getByLabel("目的地", { exact: true }).fill("杭州");
  await page.getByRole("button", { name: "开启一段出行" }).click();
  await expect(page.locator(".trip-item")).toHaveCount(2);
  await page.setViewportSize({ width: 390, height: 844 });
  const other = await downloadRecord(page);
  expect(other.text).not.toContain("杭州");
  expect(other.text).toContain("暂无已完成对话");
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
});
