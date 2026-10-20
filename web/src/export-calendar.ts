import type { Session } from "./api";

function dateValue(value: string) {
  const date = new Date(`${value}T00:00:00Z`);
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value) || value.startsWith("0000") ||
      !Number.isFinite(date.getTime()) || date.toISOString().slice(0, 10) !== value)
    throw new Error("请先保存有效的开始和结束日期");
  return date;
}

// RFC 5545 TEXT escaping prevents user text from creating calendar properties.
function text(value: string) {
  return value.replace(/\\/g, "\\\\").replace(/\r\n|\r|\n/g, "\\n")
    .replace(/;/g, "\\;").replace(/,/g, "\\,").replace(/[\u0000-\u0008\u000b\u000c\u000e-\u001f\u007f]/g, "");
}
function fold(line: string) {
  const encoder = new TextEncoder();
  let output = "", bytes = 0;
  for (const char of line) {
    const size = encoder.encode(char).length;
    if (bytes + size > 75) { output += "\r\n "; bytes = 1; }
    output += char;
    bytes += size;
  }
  return output;
}

export function tripCalendar(session: Session, now = new Date()) {
  const c = session.constraints;
  const start = dateValue(c.start_date), end = dateValue(c.end_date);
  if (start > end) throw new Error("结束日期不能早于开始日期");
  end.setUTCDate(end.getUTCDate() + 1); // DTEND is exclusive, user's end date is inclusive.
  if (end.getUTCFullYear() > 9999) throw new Error("结束日期超出日历可导出范围");
  const lines = [
    "BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//Routewise//Trip Draft//ZH", "CALSCALE:GREGORIAN",
    "BEGIN:VEVENT", `UID:trip-${encodeURIComponent(session.id)}@routewise.local`,
    `DTSTAMP:${now.toISOString().replace(/[-:]/g, "").replace(/\.\d{3}Z$/, "Z")}`,
    `DTSTART;VALUE=DATE:${c.start_date.replace(/-/g, "")}`,
    `DTEND;VALUE=DATE:${end.toISOString().slice(0, 10).replace(/-/g, "")}`,
    `SUMMARY:${text(`出行草稿 · ${session.title}`)}`, `LOCATION:${text(c.destination)}`,
    `DESCRIPTION:${text(`仅表示已保存的出行日期范围，不是预订或已确认行程。\n出发地：${c.origin || "待确认"}\n目的地：${c.destination || "待确认"}\n不包含聊天、预算或偏好；导入后请自行核对。`)}`,
    "STATUS:TENTATIVE", "TRANSP:TRANSPARENT", "CLASS:PRIVATE", "END:VEVENT", "END:VCALENDAR",
  ];
  return { filename: `行迹-出行草稿-${c.start_date}.ics`, text: lines.map(fold).join("\r\n") + "\r\n" };
}
