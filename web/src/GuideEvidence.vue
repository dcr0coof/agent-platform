<script setup lang="ts">
import { computed } from "vue";
import { parseGuideEvidence, guideFilters } from "./guide-evidence";

const props = defineProps<{ raw: string }>();
const evidence = computed(() => parseGuideEvidence(props.raw));
const filters = computed(() => guideFilters(evidence.value));
const coverage = computed(() => {
  const e = evidence.value;
  if (!e || typeof e.query_matches !== "number" || typeof e.excluded_matches !== "number" ||
      !Number.isSafeInteger(e.query_matches) || !Number.isSafeInteger(e.excluded_matches) ||
      e.excluded_matches < 0 || e.query_matches < e.excluded_matches) return null;
  const remaining = e.query_matches - e.excluded_matches;
  const omitted = remaining - e.matches.length;
  if (omitted < 0 || e.more_matches !== (omitted > 0)) return null;
  return { remaining, omitted };
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
      <p class="guide-coverage" v-if="coverage">筛选后共 {{ coverage.remaining }} 段，本次展示 {{ evidence.matches.length }} 段，未展示 {{ coverage.omitted }} 段；数量不代表相关度或资料完整性。</p>
      <p class="guide-coverage" v-else>段落覆盖数量未知：记录缺少有效一致的统计，不能按零处理。</p>
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
