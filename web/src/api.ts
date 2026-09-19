export interface Constraints {
  origin: string;
  destination: string;
  start_date: string;
  end_date: string;
  travelers: number;
  budget: number;
  budget_scope: string;
  preferences: string;
}
export interface Message {
  role: string;
  content: string;
  tool_calls?: { id: string; function: { name: string; arguments: string } }[];
  tool_call_id?: string;
}
export interface RunEvent {
  seq: number;
  type: string;
  detail: string;
  created_at: string;
}
export interface Run {
  id: string;
  session_id: string;
  input: string;
  status: string;
  error?: string;
  events: RunEvent[];
}
export interface Session {
  id: string;
  title: string;
  constraints: Constraints;
  messages: Message[];
  revision: number;
  updated_at: string;
  active_run?: Run;
}
export const blankConstraints = (): Constraints => ({
  origin: "",
  destination: "",
  start_date: "",
  end_date: "",
  travelers: 0,
  budget: 0,
  budget_scope: "",
  preferences: "",
});
export async function api<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const timeout = new AbortController();
  const signal = options.signal
    ? AbortSignal.any([options.signal, timeout.signal])
    : timeout.signal;
  const timer = setTimeout(() => timeout.abort(new Error(
    "请求超时，请检查连接。超时不代表服务端操作已取消",
  )), 15_000);
  try {
    const response = await fetch("/api" + path, {
      credentials: "same-origin",
      ...options,
      signal,
      headers: { "Content-Type": "application/json", ...options.headers },
    });
    const body = await response.json();
    if (!response.ok)
      throw new Error(body.error || `请求失败 (${response.status})`);
    return body as T;
  } catch (error) {
    // Body consumption may throw AbortError instead of the original reason.
    if (signal.aborted) throw signal.reason;
    throw error;
  } finally {
    clearTimeout(timer);
  }
}
