import type { Session } from "./api";

// Export the same completed messages and paired evidence that the page displays.
export function tripRecord(
  session: Session,
  messages: { role: string; content: string; weatherEvidence: string[] }[],
  now = new Date(),
) {
  const c = session.constraints;
  const lines = [
    `行迹 · ${session.title}`,
    `导出时间：${now.toISOString()}`,
    `记录保存时间：${session.updated_at}`,
    "仅含已保存约束和已完成对话；不含未发送消息、未保存修改及失败/取消运行。",
    "历史天气依据不代表当前天气；此记录不是预订凭证或已确认行程。",
    "",
    "=== 已保存出行约束 ===",
    `出发地：${c.origin || "待确认"}`,
    `目的地：${c.destination || "待确认"}`,
    `日期：${c.start_date || "待确认"} 至 ${c.end_date || "待确认"}`,
    `人数：${c.travelers || "待确认"}`,
    `总预算（人民币元）：${c.budget || "待确认"}`,
    `预算范围：${c.budget_scope === "all" ? "全部费用，含交通与住宿" : c.budget_scope === "local" ? "仅当地活动，不含交通住宿" : "待确认"}`,
    `偏好与要求：${c.preferences || "待确认"}`,
    "",
    "=== 已完成对话 ===",
  ];
  if (!messages.length) lines.push("暂无已完成对话。");
  for (const message of messages) {
    lines.push("", message.role === "user" ? "你：" : "行迹助手：", message.content);
    for (const evidence of message.weatherEvidence)
      lines.push("", "天气工具原始依据（可能包含未知或错误信息）：", evidence);
  }
  const title = session.title.replace(/[<>:"/\\|?*\u0000-\u001f\u007f]/g, "_").slice(0, 40);
  return {
    filename: `行迹-${title || "出行记录"}.txt`,
    text: "\uFEFF" + lines.join("\r\n") + "\r\n",
  };
}
