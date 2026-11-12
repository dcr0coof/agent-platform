<script setup lang="ts">
import { computed } from "vue";

const props = defineProps<{ raw: string }>();
type Passage = { start_line: number; end_line: number; text: string };
type Evidence = { source: string; sha256: string; notice: string; matches: Passage[]; more_matches: boolean; query_terms?: unknown; excluded_terms?: unknown };
const evidence = computed<Evidence | null>(() => {
  try {
    const value = JSON.parse(props.raw);
    if (!value || typeof value.source !== "string" || !value.source.trim() ||
      typeof value.sha256 !== "string" || !/^[a-f0-9]{64}$/.test(value.sha256) ||
      typeof value.notice !== "string" || typeof value.more_matches !== "boolean" ||
      !Array.isArray(value.matches) || value.matches.length > 3 ||
      value.matches.some((p: Passage) => !p || !Number.isSafeInteger(p.start_line) ||
        !Number.isSafeInteger(p.end_line) || p.start_line < 1 || p.end_line < p.start_line ||
        typeof p.text !== "string" || !p.text.trim())) return null;
    return value;
  } catch { return null; }
});
const filters = computed(() => {
  const query = evidence.value?.query_terms;
  const excluded = evidence.value?.excluded_terms;
  const valid = (terms: unknown): terms is string[] =>
    Array.isArray(terms) && terms.length <= 6 &&
    terms.every(term => typeof term === "string" && term.trim().length > 0);
  if (!valid(query) || query.length === 0 || !valid(excluded)) return null;
  return { query, excluded };
});
</script>

<template>
  <section class="guide-result">
    <template v-if="evidence">
      <p><strong>资料：{{ evidence.source }}</strong></p>
      <p class="guide-version">文件版本 SHA-256：{{ evidence.sha256 }}</p>
      <div class="guide-filters">
        <template v-if="filters">
          <p>实际检索词（全部匹配）：{{ JSON.stringify(filters.query) }}</p>
          <p>实际排除词（任一命中即排除整段）：{{ filters.excluded.length ? JSON.stringify(filters.excluded) : "未启用排除" }}</p>
        </template>
        <p v-else>实际筛选词未知：此记录未提供完整有效的筛选词，不等于未启用排除。</p>
      </div>
      <p>{{ evidence.notice }}</p>
      <p v-if="!evidence.matches.length">本次查询条件下没有可用段落，不能作为回答依据；不代表现实中不存在相关信息。</p>
      <section v-for="(passage, i) in evidence.matches" :key="i" class="guide-passage">
        <p>原文第 {{ passage.start_line }}–{{ passage.end_line }} 行</p>
        <pre>{{ passage.text }}</pre>
      </section>
      <p v-if="evidence.more_matches">还有其他命中段落未展示；这次仅返回原文顺序的前三段，并非相关度前三名。</p>
    </template>
    <template v-else>
      <p>无法解析攻略证据，可能是工具错误或不兼容的记录；来源与行号未知。以下为原始结果：</p>
      <pre>{{ raw }}</pre>
    </template>
  </section>
</template>
