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
  tool_calls?: unknown[];
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
  const response = await fetch("/api" + path, {
    credentials: "same-origin",
    ...options,
    headers: { "Content-Type": "application/json", ...options.headers },
  });
  const body = await response.json();
  if (!response.ok)
    throw new Error(body.error || `请求失败 (${response.status})`);
  return body as T;
}
