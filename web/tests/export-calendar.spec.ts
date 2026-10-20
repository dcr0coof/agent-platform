import { test, expect } from "@playwright/test";
import { readFile } from "node:fs/promises";
import { tripCalendar } from "../src/export-calendar";
import { blankConstraints, type Session } from "../src/api";

const session = (start: string, end: string): Session => ({
  id: "test-trip", title: "杭州", constraints: { ...blankConstraints(), start_date: start, end_date: end },
  messages: [], revision: 1, updated_at: "2026-10-20T00:00:00Z",
});

test("calendar dates are inclusive without timezone shifts and invalid dates fail", () => {
  for (const [start, end, next] of [["2028-02-28", "2028-02-29", "20280301"], ["2026-12-31", "2026-12-31", "20270101"], ["2026-03-08", "2026-03-08", "20260309"]]) {
    const record = tripCalendar(session(start, end), new Date("2026-10-20T04:00:00Z"));
    expect(record.text).toContain(`DTSTART;VALUE=DATE:${start.replace(/-/g, "")}`);
    expect(record.text).toContain(`DTEND;VALUE=DATE:${next}`);
    expect(record.text).toContain("DTSTAMP:20261020T040000Z");
  }
  for (const [start, end] of [["", ""], ["2026-02-29", "2026-03-01"], ["2026-12-02", "2026-12-01"], ["0000-01-01", "0000-01-02"], ["9999-12-31", "9999-12-31"]])
    expect(() => tripCalendar(session(start, end))).toThrow();
});

test("calendar escapes untrusted text and folds UTF-8 without injecting properties", () => {
  const s = session("2026-10-20", "2026-10-21");
  s.title = "中文🙂".repeat(30) + ",;\\\r\nEND:VEVENT\r\nBEGIN:VEVENT";
  s.constraints.destination = "杭州\nATTENDEE:someone@example.com";
  const { text } = tripCalendar(s);
  for (const line of text.split("\r\n")) expect(Buffer.byteLength(line, "utf8")).toBeLessThanOrEqual(75);
  const unfolded = text.replace(/\r\n /g, "");
  expect(unfolded.split("\r\n").filter(l => l === "BEGIN:VEVENT")).toHaveLength(1);
  expect(unfolded).toContain("\\,\\;\\\\\\nEND:VEVENT\\nBEGIN:VEVENT");
  expect(unfolded).toContain("LOCATION:杭州\\nATTENDEE:someone@example.com");
  expect(unfolded).toContain("中文🙂".repeat(30));
  expect(unfolded).not.toContain("\r\nATTENDEE:");
});

test("calendar download requires saved dates, excludes drafts and clears on trip switch", async ({ page }) => {
  await page.goto("/");
  await expect(page.locator(".trip-item")).toHaveCount(1);
  const button = page.getByRole("button", { name: "下载出行日历", exact: true });
  await expect(button).toBeDisabled();
  await page.getByLabel("出行名称", { exact: true }).fill("已保存日历计划");
  await page.getByLabel("目的地", { exact: true }).fill("杭州");
  await page.locator('input[type="date"]').nth(0).fill("2028-02-28");
  await page.locator('input[type="date"]').nth(1).fill("2028-02-29");
  await expect(button).toBeDisabled();
  await page.getByRole("button", { name: "保存出行约束" }).click();
  await expect(button).toBeEnabled();
  await page.getByLabel("出行消息").fill("不能进入日历的草稿");
  const pending = page.waitForEvent("download");
  await button.click();
  const download = await pending;
  expect(await download.failure()).toBeNull();
  expect(download.suggestedFilename()).toBe("行迹-出行草稿-2028-02-28.ics");
  const text = (await readFile((await download.path())!, "utf8")).replace(/\r\n /g, "");
  for (const value of ["DTSTART;VALUE=DATE:20280228", "DTEND;VALUE=DATE:20280301", "SUMMARY:出行草稿 · 已保存日历计划", "LOCATION:杭州", "STATUS:TENTATIVE"]) expect(text).toContain(value);
  expect(text).not.toContain("不能进入日历的草稿");
  await page.getByLabel("目的地", { exact: true }).fill("未保存北京");
  await expect(button).toBeDisabled();
  await page.getByLabel("目的地", { exact: true }).fill("杭州");
  await page.getByRole("button", { name: "开启一段出行" }).click();
  await expect(page.locator(".trip-item")).toHaveCount(2);
  await expect(button).toBeDisabled();
  await page.setViewportSize({ width: 390, height: 844 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
});
