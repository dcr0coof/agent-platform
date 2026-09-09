<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from "vue";
import {
  api,
  blankConstraints,
  type Constraints,
  type Run,
  type RunEvent,
  type Session,
} from "./api";

const sessions = ref<Session[]>([]);
const current = ref<Session | null>(null);
const form = ref<Constraints>(blankConstraints());
const title = ref("新的出行");
const input = ref("");
const demo = ref(false);
const ready = ref(false);
const error = ref("");
const notice = ref("");
const busy = ref(false);
const run = ref<Run | null>(null);
const events = ref<RunEvent[]>([]);
const connected = ref(true);
const sideOpen = ref(false);
const rightTab = ref<"constraints" | "activity">("constraints");
const feed = ref<HTMLElement | null>(null);
let stream: EventSource | null = null;
let selection = 0;
let retry: {
  id: string;
  message: string;
  sid: string;
  revision: number;
} | null = null;
const running = computed(() => run.value?.status === "running");
const dirty = computed(
  () =>
    !!current.value &&
    (title.value !== current.value.title ||
      JSON.stringify(form.value) !== JSON.stringify(current.value.constraints)),
);
const messages = computed(() =>
  (current.value?.messages || []).filter(
    (m) =>
      m.role === "user" || (m.role === "assistant" && !m.tool_calls?.length),
  ),
);
const completeCount = computed(
  () =>
    [
      form.value.origin.trim(),
      form.value.destination.trim(),
      form.value.start_date && form.value.end_date,
      form.value.travelers,
      form.value.budget,
      form.value.budget_scope,
    ].filter(Boolean).length,
);
const dateLabel = computed(() =>
  form.value.start_date
    ? `${form.value.start_date.slice(5)}${form.value.end_date ? " — " + form.value.end_date.slice(5) : ""}`
    : "日期待定",
);
const statusName: Record<string, string> = {
  running: "正在处理",
  completed: "已完成",
  cancelled: "已取消",
  failed: "执行失败",
};
const readableError = (e: unknown) =>
  e instanceof Error ? e.message : "连接异常，请稍后重试";
const reloadPage = () => window.location.reload();
const selectedTripKey = "routewise.selected-trip";
function closeStream() {
  stream?.close();
  stream = null;
}
async function scrollDown() {
  await nextTick();
  feed.value?.scrollTo({ top: feed.value.scrollHeight, behavior: "smooth" });
}
async function refreshList() {
  sessions.value = await api<Session[]>("/sessions");
}
function setSession(s: Session) {
  current.value = s;
  title.value = s.title;
  form.value = { ...s.constraints };
  try {
    sessionStorage.setItem(selectedTripKey, s.id);
  } catch {
    // Storage may be disabled; the workspace must remain usable.
  }
}
async function loadSession(id: string, checkDirty = true) {
  if (busy.value) return;
  if (
    checkDirty &&
    dirty.value &&
    !window.confirm("出行约束尚未保存，确定切换会话吗？")
  )
    return;
  const ticket = ++selection;
  closeStream();
  error.value = "";
  notice.value = "";
  sideOpen.value = false;
  try {
    const s = await api<Session>("/sessions/" + id);
    if (ticket !== selection) return;
    setSession(s);
    run.value = s.active_run || null;
    events.value = s.active_run?.events || [];
    input.value = "";
    retry = null;
    if (s.active_run) observe(s.active_run);
    await scrollDown();
  } catch (e) {
    if (ticket === selection) error.value = readableError(e);
  }
}
async function createSession() {
  if (busy.value) return;
  if (dirty.value && !window.confirm("出行约束尚未保存，确定新建会话吗？"))
    return;
  busy.value = true;
  error.value = "";
  try {
    const s = await api<Session>("/sessions", {
      method: "POST",
      body: JSON.stringify({
        title: "新的出行",
        constraints: blankConstraints(),
      }),
    });
    ++selection;
    closeStream();
    setSession(s);
    run.value = null;
    events.value = [];
    input.value = "";
    retry = null;
    sideOpen.value = false;
    await refreshList();
  } catch (e) {
    error.value = readableError(e);
  } finally {
    busy.value = false;
  }
}
async function save(): Promise<boolean> {
  if (!current.value || running.value) return false;
  const sid = current.value.id;
  try {
    const s = await api<Session>("/sessions/" + sid, {
      method: "PUT",
      body: JSON.stringify({
        title: title.value,
        constraints: form.value,
        revision: current.value.revision,
      }),
    });
    if (current.value?.id !== sid) return false;
    setSession(s);
    notice.value = "出行约束已保存";
    await refreshList();
    return true;
  } catch (e) {
    error.value = readableError(e);
    return false;
  }
}
async function saveForm() {
  if (busy.value) return;
  busy.value = true;
  error.value = "";
  try {
    await save();
  } finally {
    busy.value = false;
  }
}
async function finish(id: string, sid: string) {
  closeStream();
  try {
    const [s, r] = await Promise.all([
      api<Session>("/sessions/" + sid),
      api<Run>("/runs/" + id),
    ]);
    if (current.value?.id !== sid || run.value?.id !== id) return;
    setSession(s);
    run.value = r;
    events.value = r.events;
    connected.value = true;
    await refreshList();
    await scrollDown();
  } catch (e) {
    if (current.value?.id === sid) error.value = readableError(e);
  }
}
function observe(r: Run) {
  closeStream();
  run.value = r;
  events.value = r.events || [];
  connected.value = true;
  if (r.status !== "running") {
    void finish(r.id, r.session_id);
    return;
  }
  const last = events.value.at(-1)?.seq || 0;
  stream = new EventSource(`/api/runs/${r.id}/events?after=${last}`);
  stream.onopen = () => {
    connected.value = true;
  };
  stream.onmessage = (message) => {
    if (run.value?.id !== r.id) return;
    const event = JSON.parse(message.data) as RunEvent;
    if (!events.value.some((e) => e.seq === event.seq))
      events.value.push(event);
    if (["run.completed", "run.cancelled", "run.failed"].includes(event.type)) {
      run.value.status = event.type.slice(4);
      void finish(r.id, r.session_id);
    }
  };
  stream.onerror = () => {
    connected.value = false;
  };
}
async function send() {
  if (!current.value || busy.value || running.value || !input.value.trim())
    return;
  busy.value = true;
  error.value = "";
  notice.value = "";
  try {
    if (dirty.value && !(await save())) return;
    const sid = current.value.id,
      message = input.value.trim();
    if (!retry || retry.sid !== sid || retry.message !== message)
      retry = {
        id: crypto.randomUUID(),
        sid,
        message,
        revision: current.value.revision,
      };
    const r = await api<Run>(`/sessions/${sid}/runs`, {
      method: "POST",
      body: JSON.stringify({
        message,
        request_id: retry.id,
        revision: retry.revision,
      }),
    });
    input.value = "";
    retry = null;
    observe(r);
    rightTab.value = "activity";
    await scrollDown();
  } catch (e) {
    error.value = readableError(e) + "。可重试发送；同一请求不会重复执行。";
  } finally {
    busy.value = false;
  }
}
async function cancel() {
  if (!run.value || busy.value) return;
  const r = run.value;
  busy.value = true;
  error.value = "";
  try {
    await api<Run>(`/runs/${r.id}/cancel`, { method: "POST" });
    await finish(r.id, r.session_id);
  } catch (e) {
    error.value = readableError(e);
  } finally {
    busy.value = false;
  }
}
function usePrompt(value: string) {
  input.value = value;
}
onMounted(async () => {
  try {
    const config = await api<{ demo: boolean }>("/bootstrap");
    demo.value = config.demo;
    await refreshList();
    let selectedID: string | null = null;
    try {
      selectedID = sessionStorage.getItem(selectedTripKey);
    } catch {
      // Fall back to the latest accessible trip when storage is unavailable.
    }
    const selected = sessions.value.find((s) => s.id === selectedID);
    if (sessions.value.length)
      await loadSession((selected || sessions.value[0]).id, false);
    else await createSession();
    ready.value = true;
  } catch (e) {
    error.value = readableError(e);
  }
});
onBeforeUnmount(closeStream);
</script>

<template>
  <div class="workspace">
    <aside class="sidebar" :class="{ open: sideOpen }">
      <a class="brand" href="/" aria-label="行迹首页"
        ><span class="brand-icon">↗</span
        ><span>行迹<small>ROUTEWISE</small></span></a
      >
      <button class="new-trip" :disabled="busy" @click="createSession">
        <span>＋</span> 开启一段出行
      </button>
      <div class="nav-caption">
        我的出行 <span>{{ sessions.length.toString().padStart(2, "0") }}</span>
      </div>
      <nav class="trip-list" aria-label="出行会话">
        <button
          v-for="s in sessions"
          :key="s.id"
          :class="['trip-item', { selected: current?.id === s.id }]"
          :disabled="busy"
          @click="loadSession(s.id)"
        >
          <span class="trip-dot">↗</span
          ><span
            ><strong>{{ s.title }}</strong
            ><small
              >{{ s.constraints.destination || "目的地待定"
              }}<span class="dot">·</span>{{ s.updated_at.slice(0, 10) }}</small
            ></span
          >
        </button>
      </nav>
      <div class="sidebar-bottom">
        <span class="compass">✳</span>
        <p>留一点空间，<br />给路上的新发现。</p>
        <small>YOUR NEXT CHAPTER STARTS HERE</small>
      </div>
      <div class="profile">
        <span class="avatar">我</span
        ><span>个人工作台<small>数据保存在这台电脑</small></span
        ><span class="online-dot"></span>
      </div>
    </aside>

    <main class="main-area">
      <header class="topbar">
        <div>
          <button
            class="menu-toggle"
            aria-label="打开会话列表"
            @click="sideOpen = !sideOpen"
          >
            ☰</button
          ><span>工作台</span><span class="slash">/</span
          ><strong>出行规划</strong>
        </div>
        <span class="mode-badge"
          ><span></span>{{ demo ? "本地演示" : "模型已连接" }}</span
        >
      </header>
      <div v-if="error" class="alert" role="alert">
        {{ error
        }}<button
          @click="current ? loadSession(current.id, false) : reloadPage()"
        >
          刷新
        </button>
      </div>
      <div v-if="!ready && !error" class="loading" role="status">
        正在打开你的出行工作台…
      </div>
      <template v-if="current">
        <section class="trip-header">
          <div>
            <span class="eyebrow">A LITTLE PLANNING, A GREAT JOURNEY</span>
            <h1>{{ current.title }}</h1>
            <p>从你的约束开始，让每一步都更从容。</p>
          </div>
          <span class="saved-note"
            >◉ {{ dirty ? "有未保存修改" : "已保存到本机" }}</span
          >
        </section>
        <div class="trip-facts">
          <span>↗ {{ form.destination || "目的地待定" }}</span
          ><span>▦ {{ dateLabel }}</span
          ><span
            >♧
            {{ form.travelers ? form.travelers + " 人同行" : "人数待定" }}</span
          ><span
            >¥
            {{ form.budget ? form.budget.toLocaleString() : "预算待定" }}</span
          >
        </div>
        <div class="conversation" ref="feed">
          <section v-if="!messages.length && !run" class="welcome">
            <div class="welcome-art" aria-hidden="true">
              <span class="orbit orbit-one"></span
              ><span class="orbit orbit-two"></span
              ><span class="waypoint first"></span
              ><span class="waypoint last"></span
              ><span class="art-arrow">↗</span
              ><span class="art-note">下一站 · 由你决定</span>
            </div>
            <span class="eyebrow">LET'S MAKE A PLAN</span>
            <h2>这次，想去哪里？</h2>
            <p>
              先告诉我你的想法，再一起确认日期、预算和偏好。<br />已经确定的条件，可以直接填在右侧。
            </p>
            <div class="prompt-options">
              <button @click="usePrompt('帮我检查这次出行还缺少哪些信息。')">
                <span>01</span>检查我的出行条件 <b>↗</b></button
              ><button
                @click="usePrompt('我希望少走路、行程宽松，需要提前确认什么？')"
              >
                <span>02</span>讨论偏好与行程节奏 <b>↗</b>
              </button>
            </div>
          </section>
          <template v-for="(message, index) in messages" :key="index"
            ><article :class="['message', message.role]">
              <div class="message-avatar">
                {{ message.role === "user" ? "我" : "↗" }}
              </div>
              <div>
                <div class="message-label">
                  {{ message.role === "user" ? "你" : "行迹助手" }}
                </div>
                <p>{{ message.content }}</p>
              </div>
            </article></template
          >
          <article
            v-if="run && run.status !== 'completed'"
            class="message user"
          >
            <div class="message-avatar">我</div>
            <div>
              <div class="message-label">本次请求</div>
              <p>{{ run.input }}</p>
            </div>
          </article>
          <div v-if="running" class="run-progress" role="status">
            <span class="pulse-dot"></span
            >{{
              connected
                ? events.at(-1)?.detail || "任务已接收"
                : "连接已断开，正在续接运行记录…"
            }}<button @click="rightTab = 'activity'">查看过程 →</button>
          </div>
          <div
            v-if="run && ['failed', 'cancelled'].includes(run.status)"
            class="run-notice"
            role="status"
          >
            {{ run.error }}<small>这次请求没有加入已完成的对话历史。</small>
          </div>
        </div>
        <div class="composer-wrap">
          <form class="composer" @submit.prevent="send">
            <textarea
              v-model="input"
              aria-label="出行消息"
              placeholder="说说你的出行想法，或继续补充一个条件…"
              maxlength="5000"
              :disabled="running || busy"
              @keydown.enter.exact.prevent="!$event.isComposing && send()"
            ></textarea>
            <div class="composer-bottom">
              <span>{{
                demo
                  ? "演示回复 · 不调用模型、天气或路线"
                  : "依据已确认约束进行对话"
              }}</span
              ><button
                v-if="running"
                type="button"
                class="cancel-button"
                :disabled="busy"
                @click="cancel"
              >
                ■ 停止任务</button
              ><button
                v-else
                type="submit"
                class="send-button"
                :disabled="!input.trim() || busy"
                aria-label="发送消息"
              >
                发送 <span>↑</span>
              </button>
            </div>
          </form>
          <p class="composer-caption">
            Enter 发送 · Shift + Enter 换行<span>重要条件请在右侧确认保存</span>
          </p>
        </div>
      </template>
    </main>

    <aside v-if="current" class="details">
      <div class="detail-tabs" role="tablist" aria-label="出行详情">
        <button
          :class="{ active: rightTab === 'constraints' }"
          role="tab"
          :aria-selected="rightTab === 'constraints'"
          @click="rightTab = 'constraints'"
        >
          出行约束</button
        ><button
          :class="{ active: rightTab === 'activity' }"
          role="tab"
          :aria-selected="rightTab === 'activity'"
          @click="rightTab = 'activity'"
        >
          执行过程 <span v-if="events.length">{{ events.length }}</span>
        </button>
      </div>
      <section v-if="rightTab === 'constraints'" class="constraints-panel">
        <div class="panel-heading">
          <h2>把确定的事，先记下来</h2>
          <p>未知的条件可以留空，助手会继续询问。</p>
        </div>
        <div class="completion">
          <span>已确认 {{ completeCount }} / 6 项</span>
          <div class="segments">
            <i
              v-for="i in 6"
              :key="i"
              :class="{ filled: i <= completeCount }"
            ></i>
          </div>
        </div>
        <form @submit.prevent="saveForm">
          <fieldset :disabled="running || busy">
            <label
              >出行名称<input
                v-model="title"
                maxlength="80"
                placeholder="给这段旅程起个名字"
            /></label>
            <div class="form-row">
              <label
                >出发地<input
                  v-model="form.origin"
                  placeholder="从哪里出发"
                  maxlength="60" /></label
              ><label
                >目的地<input
                  v-model="form.destination"
                  placeholder="想去哪里"
                  maxlength="60"
              /></label>
            </div>
            <label
              >出行日期
              <div class="form-row">
                <input
                  v-model="form.start_date"
                  type="date"
                  aria-label="开始日期"
                /><input
                  v-model="form.end_date"
                  type="date"
                  aria-label="结束日期"
                  :min="form.start_date"
                /></div
            ></label>
            <div class="form-row">
              <label
                >出行人数<input
                  :value="form.travelers || ''"
                  @input="
                    form.travelers = Number(
                      ($event.target as HTMLInputElement).value,
                    )
                  "
                  type="number"
                  min="1"
                  max="100"
                  placeholder="待确认" /></label
              ><label
                >总预算（元）<input
                  :value="form.budget || ''"
                  @input="
                    form.budget = Number(
                      ($event.target as HTMLInputElement).value,
                    )
                  "
                  type="number"
                  min="0"
                  max="10000000"
                  placeholder="待确认"
              /></label>
            </div>
            <label
              >预算包含什么<select v-model="form.budget_scope">
                <option value="">待确认</option>
                <option value="all">全部费用，含交通与住宿</option>
                <option value="local">仅当地活动，不含交通住宿</option>
              </select></label
            ><label
              >偏好与特别要求<textarea
                v-model="form.preferences"
                maxlength="1200"
                rows="3"
                placeholder="例如：不自驾、少走路、喜欢自然景观…"
              ></textarea></label
            ><button class="save-button" type="submit" :disabled="!dirty">
              {{ dirty ? "保存出行约束" : "约束已保存" }} <span>✓</span>
            </button>
          </fieldset>
          <p v-if="notice" class="notice" role="status">{{ notice }}</p>
        </form>
        <div class="quiet-note">
          <span>↗</span>
          <p>你始终掌握决定权。<br />聊天中的新偏好，请确认后同步到这里。</p>
        </div>
      </section>
      <section v-else class="activity-panel">
        <div class="panel-heading">
          <h2>每一步，都有记录</h2>
          <p>这里只展示真实操作与状态，不展示模型内部推理。</p>
        </div>
        <div v-if="run" class="run-summary">
          <span :class="['status-dot', run.status]"></span
          >{{ statusName[run.status] || run.status
          }}<small>{{ demo ? "本地演示运行" : "模型工具运行" }}</small>
        </div>
        <ol v-if="events.length" class="timeline">
          <li v-for="event in events" :key="event.seq">
            <span class="timeline-mark"></span>
            <div>
              <strong>{{ event.detail }}</strong
              ><small
                >{{
                  new Date(event.created_at).toLocaleTimeString("zh-CN", {
                    hour12: false,
                  })
                }}
                <span>· {{ event.type }}</span></small
              >
            </div>
          </li>
        </ol>
        <div v-else class="empty-activity">
          <span>◎</span>
          <p>发送第一条消息后，<br />这里会记录任务的执行过程。</p>
        </div>
      </section>
      <div class="details-footer"><span>↗</span> 行迹 · 为下一程留好空间</div>
    </aside>
  </div>
</template>
