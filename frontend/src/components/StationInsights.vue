<script setup lang="ts">
import { computed } from 'vue'
import { Activity, ArrowUpRight, Clock3, Info, RefreshCw, Trophy, X } from 'lucide-vue-next'
import { availabilityLabels, captureFreshness } from '../insights'
import type { Availability, Station, StationFilters, StationSummary } from '../types'

const props = defineProps<{
  summary: StationSummary | null
  filters: StationFilters
  loading: boolean
  error: string
}>()
const emit = defineEmits<{
  select: [station: Station]
  filterAvailability: [availability: Availability | '']
  retry: []
}>()
const categories: { key: Availability; hint: string }[] = [
  { key: 'idle', hint: '空闲率 ≥ 50%' },
  { key: 'moderate', hint: '20% ≤ 空闲率 < 50%' },
  { key: 'full', hint: '空闲率 < 20%' },
  { key: 'unknown', hint: '无电桩明细或电桩总数为 0' },
]
const scope = computed(() => [
  props.filters.city || '全部城市', props.filters.operator,
  props.filters.keyword && `搜索“${props.filters.keyword}”`,
  ({ cheap: '低价', mid: '中位价', expensive: '高价' } as Record<string, string>)[props.filters.priceBand],
  ({ has: '有多时段', flat: '全天统一价', none: '无分时数据' } as Record<string, string>)[props.filters.tou],
  ({ with: '有电桩明细', without: '无电桩明细' } as Record<string, string>)[props.filters.piles],
].filter(Boolean).join(' · '))
const freshnessHours = computed(() => Math.round((props.summary?.freshnessWindowSeconds ?? 86400) / 3600))
const staleRecommendations = computed(() => props.summary?.topAvailable.some(station => freshness(station) !== 'recent'))
function count(key: Availability) { return props.summary?.availability[key] ?? 0 }
function percent(key: Availability) { return props.summary?.total ? count(key) * 100 / props.summary.total : 0 }
function toggle(key: Availability) {
  emit('filterAvailability', props.filters.availability === key ? '' : key)
}
function freshness(station: Station) { return props.summary ? captureFreshness(station.capturedAt, props.summary) : 'unknown' }
function captureLabel(station: Station) {
  return { recent: `${freshnessHours.value}h 内采集`, stale: '采集待更新', unknown: '采集时间未知' }[freshness(station)]
}
function captureTitle(station: Station) {
  if (freshness(station) === 'unknown') return '缺少有效采集时间，不能判断新鲜度'
  return `采集于 ${new Date(station.capturedAt * 1000).toLocaleString('zh-CN', { hour12: false })}`
}
</script>

<template>
  <aside class="detail-panel insights-panel" aria-label="当前筛选概览" :aria-busy="loading">
    <header class="insights-header">
      <div class="insights-heading"><span class="insights-mark"><Activity :size="16" /></span><h2>当前筛选概览</h2></div>
      <p class="insights-scope" :title="scope">{{ scope }}</p>
      <div class="insights-caption">
        <span v-if="summary && !loading && !error">{{ summary.total.toLocaleString() }} 个站点 · 完整筛选结果</span>
        <span v-else>统计覆盖全部匹配站点，不限当前页</span>
      </div>
      <button v-if="filters.availability" class="insights-filter-chip" @click="emit('filterAvailability', '')">
        仅看{{ availabilityLabels[filters.availability] }} <X :size="12" /><span class="sr-only">，清除状态筛选</span>
      </button>
    </header>

    <div v-if="loading" class="insights-loading" role="status">
      <span>正在汇总筛选结果…</span>
      <div class="insights-skeleton summary-skeleton-bar"></div>
      <div v-for="index in 6" :key="index" class="insights-skeleton" aria-hidden="true"></div>
    </div>
    <div v-else-if="error || !summary" class="insights-error" role="status">
      <Info :size="24" />
      <strong>概览暂时不可用</strong>
      <span>{{ error || '未能获取完整统计，请重试。' }}</span>
      <button class="insights-retry" @click="emit('retry')"><RefreshCw :size="13" />重新加载</button>
    </div>
    <template v-else>
      <section class="insights-section" aria-labelledby="availability-heading">
        <div class="insights-section-heading"><h3 id="availability-heading"><Activity :size="14" />站点空闲分布</h3><span>点击状态筛选</span></div>
        <div class="availability-bar" aria-label="站点状态占比分布">
          <button
            v-for="item in categories.filter(item => count(item.key) > 0)" :key="item.key"
            :class="`availability-${item.key}`" :style="{ width: `${percent(item.key)}%` }"
            :aria-label="`${availabilityLabels[item.key]} ${count(item.key)} 站，占 ${percent(item.key).toFixed(1)}%`"
            :title="`${availabilityLabels[item.key]} ${percent(item.key).toFixed(1)}% · ${item.hint}`"
            :aria-pressed="filters.availability === item.key" @click="toggle(item.key)"
          ></button>
        </div>
        <div class="availability-grid">
          <button
            v-for="item in categories" :key="item.key" class="availability-stat" :class="[{ active: filters.availability === item.key }, `availability-${item.key}`]"
            :aria-pressed="filters.availability === item.key" :title="item.hint"
            @click="toggle(item.key)"
          >
            <span><i></i>{{ availabilityLabels[item.key] }}</span><strong>{{ count(item.key).toLocaleString() }}<small>站</small></strong>
          </button>
        </div>
        <p v-if="!summary.total" class="insights-empty-note">没有符合当前条件的站点{{ filters.availability ? '，可清除状态筛选重试' : '' }}。</p>
      </section>

      <section class="insights-section" aria-labelledby="recommendations-heading">
        <div class="insights-section-heading"><h3 id="recommendations-heading"><Trophy :size="14" />低价可充站 <em>TOP 5</em></h3><ArrowUpRight :size="14" /></div>
        <p class="insights-section-note">有空闲桩 · 电价优先，同价空闲桩多的优先</p>
        <div v-if="!summary.topAvailable.length" class="insights-empty-recommendations">
          <Info :size="19" /><span>暂无有电价且有空闲桩的站点</span><small>可尝试放宽筛选条件</small>
        </div>
        <ol v-else class="recommendation-list">
          <li v-for="(station, index) in summary.topAvailable" :key="station.sourceKey">
            <button class="recommendation-row" :title="station.matchedName || station.requestedName" @click="emit('select', station)">
              <span class="recommendation-rank" :class="{ leading: index === 0 }">{{ String(index + 1).padStart(2, '0') }}</span>
              <span class="recommendation-body">
                <span class="recommendation-main"><strong>{{ station.matchedName || station.requestedName || '未命名站点' }}</strong><b>{{ station.currentPriceText || station.currentPrice.toFixed(2) }}<small>元/kWh</small></b></span>
                <span class="recommendation-location">{{ [station.city, station.district].filter(Boolean).join(' · ') || '地区未知' }}</span>
                <span class="recommendation-meta"><span>空闲 {{ station.pileIdle }}/{{ station.pileTotal }} 根 · {{ station.idleRate.toFixed(0) }}%</span><small :class="`capture-${freshness(station)}`" :title="captureTitle(station)">{{ captureLabel(station) }}</small></span>
              </span>
            </button>
          </li>
        </ol>
        <p v-if="staleRecommendations" class="insights-stale-note"><Info :size="12" />榜单含待更新或时间未知的数据，使用前请确认。</p>
      </section>

      <section class="insights-section insights-freshness" aria-labelledby="freshness-heading">
        <div class="insights-section-heading"><h3 id="freshness-heading"><Clock3 :size="14" />数据新鲜度</h3><span>按站点采集时间</span></div>
        <div class="freshness-grid">
          <div class="freshness-recent"><strong>{{ summary.freshness.recent.toLocaleString() }}</strong><span>{{ freshnessHours }}h 内采集</span></div>
          <div class="freshness-stale"><strong>{{ summary.freshness.stale.toLocaleString() }}</strong><span>超 {{ freshnessHours }}h 待更新</span></div>
          <div><strong>{{ summary.freshness.unknown.toLocaleString() }}</strong><span>时间未知</span></div>
        </div>
        <div class="insights-quality"><span>缺少电价 <b>{{ summary.freshness.missingPrice.toLocaleString() }}</b></span><span>无电桩数据 <b>{{ summary.freshness.missingPiles.toLocaleString() }}</b></span></div>
        <p class="insights-section-note">页面刷新不代表重新采集；缺失项可能重叠。</p>
      </section>
      <footer class="insights-footer"><Info :size="12" />点击站点查看详情，关闭后返回此概览</footer>
    </template>
  </aside>
</template>
