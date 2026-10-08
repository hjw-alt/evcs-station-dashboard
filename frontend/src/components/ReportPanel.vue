<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { fetchAIReportDates, fetchDailyAIReport } from '../api'
import { renderMarkdown } from '../markdown'
import type { DailyAIReport, Overview, StationFilters, StationSummary } from '../types'

const props = defineProps<{
  overview: Overview | null
  summary: StationSummary | null
  filters: StationFilters
  loading: boolean
}>()

const scope = computed(() => [
  props.filters.city || '全省',
  props.filters.operator,
  props.filters.keyword && `搜索"${props.filters.keyword}"`,
].filter(Boolean).join(' · '))

const total = computed(() => props.summary?.total ?? props.overview?.total ?? 0)
const idleRate = computed(() => props.overview?.idleRate ?? 0)
const busyRate = computed(() => props.overview?.busyRate ?? 0)
const idlePile = computed(() => props.overview?.pileIdle ?? 0)
const busyPile = computed(() => props.overview?.pileBusy ?? 0)
const pileTotal = computed(() => props.overview?.pileTotal ?? 0)
const averagePrice = computed(() => props.overview?.averagePrice ?? 0)
const lastUpdated = computed(() => props.summary?.asOf || props.overview?.updatedAt || 0)
const aiReport = ref<DailyAIReport | null>(null)
const aiLoading = ref(false)
const aiError = ref('')
const reportDates = ref<{ date: string; status: string }[]>([])
const selectedDate = ref('')
// 模型返回的是 Markdown，直接插值会把 # / ** / | 原样显示
const aiHtml = computed(() => renderMarkdown(aiReport.value?.content ?? ''))

async function loadReportDates() {
  try {
    const response = await fetchAIReportDates()
    reportDates.value = response.items || []
    if (!selectedDate.value && reportDates.value.length > 0) {
      selectedDate.value = reportDates.value[0].date
    }
  } catch {
    reportDates.value = []
  }
}

async function loadAIReport(date = selectedDate.value) {
  aiLoading.value = true
  aiError.value = ''
  try {
    aiReport.value = await fetchDailyAIReport(date || undefined)
    if (!selectedDate.value && aiReport.value?.reportDate) {
      selectedDate.value = aiReport.value.reportDate
    }
  } catch (error) {
    aiError.value = error instanceof Error ? error.message : String(error)
  } finally {
    aiLoading.value = false
  }
}

const statusRows = computed(() => {
  const availability = props.summary?.availability
  return [
    { label: '空闲站点', count: availability?.idle ?? 0, rule: '空闲率 ≥ 50%' },
    { label: '适中站点', count: availability?.moderate ?? 0, rule: '20% ≤ 空闲率 < 50%' },
    { label: '满载站点', count: availability?.full ?? 0, rule: '空闲率 < 20%' },
    { label: '无明细站点', count: availability?.unknown ?? 0, rule: '缺少电桩状态明细' },
  ]
})

const assessment = computed(() => {
  if (idleRate.value >= 60) {
    return {
      level: '供给充足',
      text: '充电桩整体空闲水平较高，重卡补能压力较小。当前可重点观察区域间差异，避免少数高负载站点被平均值掩盖。',
    }
  }
  if (idleRate.value >= 35) {
    return {
      level: '供需平稳',
      text: '充电资源处于可用区间，能够支撑日常补能需求。建议继续按日观察空闲率变化，重点关注集中补能时段。',
    }
  }
  return {
    level: '供给偏紧',
    text: '空闲资源相对不足，高负载站点可能增加排队概率。建议优先核查满载站点，并结合区域分布评估扩容或调度空间。',
  }
})

function fmt(value: number | undefined, digits = 1) {
  return value == null ? '--' : value.toFixed(digits)
}

function fmtPercent(value: number, digits = 1) {
  return `${value.toFixed(digits)}%`
}

function fmtCount(value: number | undefined) {
  return (value ?? 0).toLocaleString()
}

function reportDate(epoch: number) {
  if (!epoch) return '--'
  return new Date(epoch * 1000).toLocaleDateString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit' })
}

function reportTime(epoch: number) {
  if (!epoch) return '--'
  return new Date(epoch * 1000).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false })
}

onMounted(async () => {
  await loadReportDates()
  await loadAIReport(selectedDate.value)
})
watch(selectedDate, (value) => {
  if (value && value !== aiReport.value?.reportDate) void loadAIReport(value)
})
</script>

<template>
  <div class="report-view">
    <div v-if="loading" class="report-loading" role="status">
      <div class="report-skeleton report-skeleton-hero"></div>
      <div class="report-skeleton report-skeleton-text"></div>
      <div class="report-skeleton report-skeleton-text short"></div>
      <div class="report-skeleton report-skeleton-text"></div>
    </div>

    <article v-else class="report-daily">
      <header class="daily-header">
        <div>
          <span class="daily-kicker">每日运行报告 · {{ scope }}</span>
          <h1>重卡充电站空闲率概览</h1>
          <p>本报告按采集日汇总，展示当前筛选范围内重卡充电站的整体空闲水平、占用状态和运营判断。</p>
        </div>
        <div class="daily-date">
          <span>报告日期</span>
          <strong>{{ reportDate(lastUpdated) }}</strong>
          <small>数据时点 {{ reportTime(lastUpdated) }}</small>
        </div>
      </header>

      <section class="daily-core">
        <div class="daily-idle">
          <span>电桩空闲率</span>
          <strong>{{ fmt(idleRate) }}<i>%</i></strong>
          <small>空闲 {{ fmtCount(idlePile) }} 根 / 忙碌 {{ fmtCount(busyPile) }} 根</small>
        </div>
        <div class="daily-summary-text">
          <p><strong>{{ assessment.level }}</strong>。{{ assessment.text }}</p>
          <p>
            本期纳入 {{ fmtCount(total) }} 个重卡充电站，涉及电桩 {{ fmtCount(pileTotal) }} 根。
            空闲率为 {{ fmt(idleRate) }}%，忙碌率为 {{ fmt(busyRate) }}%，
            平均电价参考值为 {{ fmt(averagePrice, 2) }} 元/kWh。该指标用于快速判断当前采集周期内的补能资源是否充足。
          </p>
        </div>
      </section>

      <section class="daily-section daily-ai-section">
        <div class="section-head">
          <h2>AI 每日运行分析</h2>
          <label class="ai-report-date-select">
            <span>报告日期</span>
            <select v-model="selectedDate" :disabled="aiLoading || reportDates.length === 0">
              <option v-for="item in reportDates" :key="item.date" :value="item.date">{{ item.date }}</option>
            </select>
          </label>
          <span class="report-section-note">每天自动生成</span>
        </div>
        <div v-if="aiLoading" class="ai-report-status">AI 报告加载中...</div>
        <div v-else-if="aiError" class="ai-report-status error">{{ aiError }}</div>
        <div v-else-if="aiReport?.status === 'COMPLETED' && aiReport.content" class="ai-report-content" v-html="aiHtml"></div>
        <div v-else class="ai-report-status">
          AI 报告尚未生成或暂不可用。
          <small v-if="aiReport?.status === 'FAILED' && aiReport.error">{{ aiReport.error }}</small>
        </div>
      </section>

      <section class="daily-section daily-collection">
        <h2>一、采集口径</h2>
        <p>
          后台采集终端约一天完成一轮站点循环。每一轮会重新读取站点详情、电价信息和电桩状态，
          并把结果写入最新站点数据和历史快照。本报告使用的是当前筛选范围内最近一轮采集后的汇总结果。
        </p>
        <p>
          如果个别站点在本轮搜索失败、页面加载异常或缺少电桩明细，其状态会归入“无明细站点”，
          后续轮次仍会继续尝试补采。因此，这里的汇总反映的是当前采集周期结束后的可用快照，而不是单次实时瞬时值。
        </p>
      </section>

      <section class="daily-section daily-status">
        <h2>二、空闲状态分布</h2>
        <p>
          空闲率按站点已有电桩明细计算。空闲站点表示仍有较充足的可用充电能力；
          适中站点表示部分电桩已被占用；满载站点表示可用资源较少，可能存在排队风险；
          无明细站点通常是本轮没有成功读取电桩列表，或站点页面未展示逐桩状态。
        </p>
        <div class="daily-status-list">
          <div v-for="row in statusRows" :key="row.label" class="daily-status-row">
            <span>{{ row.label }}</span>
            <strong>{{ fmtCount(row.count) }}</strong>
            <small>{{ row.rule }}</small>
          </div>
        </div>
      </section>

      <section class="daily-section daily-operations">
        <h2>三、运营观察</h2>
        <p>
          从总量看，全网空闲率为 {{ fmt(idleRate) }}%，忙碌率为 {{ fmt(busyRate) }}%。
          若空闲率持续下降，说明重卡补能需求增长快于供给释放；若空闲率稳定在较高水平，
          则说明当前资源能够覆盖常用运行线路和集中补能时段。
        </p>
        <p>
          后续建议重点关注三类对象：一是连续满载的站点，优先核查排队、故障和扩容空间；
          二是无明细站点，确认是否存在页面结构变化或采集失败；三是平均电价明显偏高的区域，
          结合空闲率判断价格与可用资源之间的关系。
        </p>
      </section>

      <footer class="daily-footer">
        本报告由重卡充电站监控台根据最新采集结果自动汇总。
      </footer>
    </article>
  </div>
</template>
