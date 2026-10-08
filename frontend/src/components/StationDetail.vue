<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { LineChart } from 'echarts/charts'
import { DataZoomComponent, GridComponent, TitleComponent, TooltipComponent } from 'echarts/components'
import * as echarts from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { Activity, BatteryCharging, Clock3, MapPin, X, Zap } from 'lucide-vue-next'
import type { HistoryPoint, PileDetail, PricePeriod, StationDetail } from '../types'
import { readChartTheme } from '../theme'

echarts.use([LineChart, DataZoomComponent, GridComponent, TooltipComponent, TitleComponent, CanvasRenderer])

const props = defineProps<{
  detail: StationDetail | null
  history: HistoryPoint[]
  loading: boolean
}>()

const emit = defineEmits<{ close: [] }>()

const priceChartRef = ref<HTMLDivElement | null>(null)
const historyChartRef = ref<HTMLDivElement | null>(null)
let priceChart: echarts.EChartsType | null = null
let historyChart: echarts.EChartsType | null = null
let resizeObserver: ResizeObserver | null = null
let chartRetryTimer: number | undefined

const periods = computed<PricePeriod[]>(() => {
  if (!props.detail) return []
  return [...props.detail.fastPrices, ...props.detail.slowPrices]
})

// 电价趋势固定为"昨日 + 今日"各 5 个分时时段（与高德详情页一致），
// 同一时段内多次采集只取第一次的价格（同一时段电价相同）。
type PriceSlot = {
  shortLabel: string
  dayLabel: string
  windowLabel: string
  price: number | null
  capturedAt: number
}

const PRICE_WINDOWS: { label: string; startMin: number; endMin: number }[] = [
  { label: '00:00-06:00', startMin: 0, endMin: 360 },
  { label: '06:00-11:00', startMin: 360, endMin: 660 },
  { label: '11:00-14:00', startMin: 660, endMin: 840 },
  { label: '14:00-16:00', startMin: 840, endMin: 960 },
  { label: '16:00-23:59', startMin: 960, endMin: 1440 },
]

function buildPriceSlots(points: HistoryPoint[]): PriceSlot[] {
  const slots: PriceSlot[] = []
  const now = new Date()
  const dayStart = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime()
  const days = [
    { label: '昨日', base: dayStart - 24 * 3600 * 1000 },
    { label: '今日', base: dayStart },
  ]
  for (const day of days) {
    const dateLabel = new Date(day.base).toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' })
    for (const window of PRICE_WINDOWS) {
      const start = day.base + window.startMin * 60 * 1000
      const end = day.base + window.endMin * 60 * 1000
      const first = points
        .filter(item => {
          const ts = item.capturedAt * 1000
          return ts >= start && ts < end
        })
        .sort((a, b) => a.capturedAt - b.capturedAt)[0]
      slots.push({
        shortLabel: `${dateLabel} ${window.label.slice(0, 5)}`,
        dayLabel: `${day.label} ${dateLabel}`,
        windowLabel: window.label,
        price: first && first.currentPrice > 0 ? first.currentPrice : null,
        capturedAt: first ? first.capturedAt : 0,
      })
    }
  }
  return slots
}

function numeric(value: string) {
  const number = Number.parseFloat(value)
  return Number.isFinite(number) ? number : null
}

function renderPriceChart() {
  if (!priceChartRef.value) return
  if (priceChartRef.value.clientWidth < 20 || priceChartRef.value.clientHeight < 20) {
    scheduleChartRetry()
    return
  }
  if (priceChart && priceChart.getDom() !== priceChartRef.value) {
    priceChart.dispose()
    priceChart = null
  }
  priceChart ??= echarts.init(priceChartRef.value)
  priceChart.resize()
  const theme = readChartTheme()
  const pricePoints = props.history.filter(item => item.currentPrice > 0)
  if (!pricePoints.length) {
    // 当前采集模式每轮只在列表页取一个电价点；没有趋势数据时，
    // 退回展示历史上从详情页采集到的分时价格阶梯。
    if (periods.value.length) {
      renderPriceLadder(theme)
      return
    }
    priceChart.clear()
    priceChart.setOption({ backgroundColor: 'transparent', title: { text: '暂无电价数据', left: 'center', top: 'middle', textStyle: { color: theme.muted, fontSize: 12, fontWeight: 400 } } })
    return
  }
  const slots = buildPriceSlots(pricePoints)
  priceChart.setOption({
    animationDuration: 400,
    grid: { left: 48, right: 16, top: 24, bottom: 36 },
    tooltip: {
      trigger: 'axis',
      backgroundColor: theme.panel,
      borderColor: theme.lineStrong,
      textStyle: { color: theme.text, fontSize: 11 },
      formatter: (params: unknown) => {
        const list = Array.isArray(params) ? params : [params]
        const first = (list[0] ?? {}) as { dataIndex?: number }
        const index = typeof first.dataIndex === 'number' ? first.dataIndex : -1
        const slot = index >= 0 ? slots[index] : undefined
        if (!slot) return ''
        if (slot.price == null) return `${slot.dayLabel} ${slot.windowLabel}<br/>未采集`
        const time = slot.capturedAt
          ? new Date(slot.capturedAt * 1000).toLocaleString('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false })
          : ''
        return `${slot.dayLabel} ${slot.windowLabel}<br/>电价 ${slot.price} 元/kWh${time ? `<br/>采集于 ${time}` : ''}`
      },
    },
    xAxis: {
      type: 'category',
      data: slots.map(item => item.shortLabel),
      axisLabel: { color: theme.muted, fontSize: 9, interval: 0, rotate: 22 },
      axisLine: { lineStyle: { color: theme.lineStrong } },
    },
    yAxis: {
      type: 'value',
      axisLabel: { color: theme.muted, fontSize: 9, formatter: '{value}' },
      splitLine: { lineStyle: { color: theme.line } },
    },
    series: [{
      name: '电价',
      type: 'line',
      connectNulls: false,
      symbolSize: 6,
      data: slots.map(item => item.price),
      lineStyle: { color: theme.primary, width: 2 },
      itemStyle: { color: theme.primary },
      areaStyle: { color: theme.primaryArea },
    }],
  }, true)
}

function renderPriceLadder(theme: ReturnType<typeof readChartTheme>) {
  if (!priceChart) return
  priceChart.setOption({
    animationDuration: 400,
    title: { text: '历史分时价格', left: 'center', top: 0, textStyle: { color: theme.muted, fontSize: 11, fontWeight: 400 } },
    grid: { left: 44, right: 12, top: 30, bottom: 28 },
    tooltip: { trigger: 'axis', backgroundColor: theme.panel, borderColor: theme.lineStrong, textStyle: { color: theme.text, fontSize: 11 } },
    dataZoom: [],
    xAxis: {
      type: 'category',
      data: periods.value.map(item => item.time || '当前时段'),
      axisLabel: { color: theme.muted, fontSize: 10, interval: 0, rotate: periods.value.length > 4 ? 18 : 0 },
      axisLine: { lineStyle: { color: theme.lineStrong } },
    },
    yAxis: {
      type: 'value',
      axisLabel: { color: theme.muted, fontSize: 10 },
      splitLine: { lineStyle: { color: theme.line } },
    },
    series: [{
      type: 'line',
      step: 'end',
      symbolSize: 7,
      data: periods.value.map(item => numeric(item.totalPrice)),
      lineStyle: { color: theme.primary, width: 2 },
      itemStyle: { color: theme.primary },
      areaStyle: { color: theme.primaryArea },
    }],
  }, true)
}

function renderHistoryChart() {
  if (!historyChartRef.value) return
  if (historyChartRef.value.clientWidth < 20 || historyChartRef.value.clientHeight < 20) {
    scheduleChartRetry()
    return
  }
  if (historyChart && historyChart.getDom() !== historyChartRef.value) {
    historyChart.dispose()
    historyChart = null
  }
  historyChart ??= echarts.init(historyChartRef.value)
  historyChart.resize()
  const theme = readChartTheme()
  if (!props.history.length) {
    historyChart.clear()
    historyChart.setOption({ backgroundColor: 'transparent', title: { text: '暂无历史快照', left: 'center', top: 'middle', textStyle: { color: theme.muted, fontSize: 12, fontWeight: 400 } } })
    return
  }
  const historyWindowSize = 24
  const historyStart = props.history.length > historyWindowSize
    ? Math.max(0, 100 - historyWindowSize * 100 / props.history.length)
    : 0
  historyChart.setOption({
    animationDuration: 400,
    grid: { left: 44, right: 16, top: 20, bottom: 58 },
    dataZoom: [
      {
        type: 'inside',
        start: historyStart,
        end: 100,
        zoomOnMouseWheel: true,
        moveOnMouseMove: true,
      },
      {
        type: 'slider',
        start: historyStart,
        end: 100,
        bottom: 4,
        height: 16,
        borderColor: theme.lineStrong,
        backgroundColor: theme.surface,
        fillerColor: theme.primarySelection,
        handleStyle: { color: theme.primary, borderColor: theme.primary },
        moveHandleStyle: { color: theme.primary },
        textStyle: { color: theme.muted, fontSize: 9 },
        dataBackground: { lineStyle: { color: theme.lineStrong }, areaStyle: { color: theme.line } },
        selectedDataBackground: { lineStyle: { color: theme.primary }, areaStyle: { color: theme.primarySelection } },
      },
    ],
    tooltip: { trigger: 'axis', backgroundColor: theme.panel, borderColor: theme.lineStrong, textStyle: { color: theme.text, fontSize: 11 } },
    xAxis: {
      type: 'category',
      data: props.history.map(item => new Date(item.capturedAt * 1000).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false })),
      axisLabel: { color: theme.muted, fontSize: 9, hideOverlap: true },
      axisLine: { lineStyle: { color: theme.lineStrong } },
    },
    yAxis: { type: 'value', min: 0, max: 100, axisLabel: { color: theme.muted, fontSize: 9, formatter: '{value}%' }, splitLine: { lineStyle: { color: theme.line } } },
    series: [
      { name: '空闲率', type: 'line', smooth: true, data: props.history.map(item => item.pileIdle + item.pileBusy > 0 ? Math.round(item.pileIdle * 100 / (item.pileIdle + item.pileBusy)) : null), symbolSize: 4, lineStyle: { color: theme.green, width: 2 }, itemStyle: { color: theme.green } },
    ],
  }, true)
}

function scheduleChartRetry() {
  if (chartRetryTimer) window.clearTimeout(chartRetryTimer)
  chartRetryTimer = window.setTimeout(() => {
    renderPriceChart()
    renderHistoryChart()
  }, 120)
}

async function refreshCharts() {
  await nextTick()
  if (!props.detail) {
    priceChart?.dispose()
    historyChart?.dispose()
    priceChart = null
    historyChart = null
    return
  }
  window.requestAnimationFrame(() => {
    renderPriceChart()
    renderHistoryChart()
    observeChartContainers()
  })
}

function observeChartContainers() {
  if (typeof ResizeObserver === 'undefined') return
  resizeObserver ??= new ResizeObserver(() => {
    priceChart?.resize()
    historyChart?.resize()
  })
  if (priceChartRef.value) resizeObserver.observe(priceChartRef.value)
  if (historyChartRef.value) resizeObserver.observe(historyChartRef.value)
}

watch(() => [props.detail?.sourceKey, props.history.length, props.loading], () => void refreshCharts())
onMounted(() => {
  void refreshCharts()
  window.addEventListener('resize', refreshCharts)
})
onBeforeUnmount(() => {
  window.removeEventListener('resize', refreshCharts)
  if (chartRetryTimer) window.clearTimeout(chartRetryTimer)
  resizeObserver?.disconnect()
  priceChart?.dispose()
  historyChart?.dispose()
})

function fmtTime(epoch: number) {
  if (!epoch) return '--'
  return new Date(epoch * 1000).toLocaleString('zh-CN', { hour12: false })
}

function pileGroup(pile: PileDetail) {
  if (pile.chargingType.includes('超')) return '超充'
  if (pile.chargingType.includes('慢')) return '慢充'
  return '快充'
}
</script>

<template>
  <aside class="detail-panel">
    <button v-if="detail || loading" class="detail-close icon-button" title="关闭详情" aria-label="关闭详情" @click="emit('close')"><X :size="16" /></button>

    <div v-if="loading" class="detail-empty" role="status" aria-live="polite">
      <BatteryCharging :size="34" />
      <strong>正在加载站点详情…</strong>
      <span>正在获取电价、空闲率和历史快照</span>
    </div>

    <div v-else-if="!detail" class="detail-empty">
      <BatteryCharging :size="34" />
      <strong>选择站点查看详情</strong>
      <span>电价趋势、空闲率趋势和电桩静态信息会显示在这里</span>
    </div>

    <template v-else-if="detail">
      <div class="detail-title">
        <div class="detail-kicker">{{ detail.city }}{{ detail.district ? ` · ${detail.district}` : '' }}</div>
        <h2>{{ detail.matchedName }}</h2>
        <p><MapPin :size="12" />{{ detail.address || detail.sourceStationId }}</p>
      </div>

      <div class="detail-price-row">
        <div>
          <span>当前电价</span>
          <strong>{{ detail.currentPriceText || '--' }}<em v-if="detail.currentPriceText">元/kWh</em></strong>
        </div>
      </div>

      <section class="detail-section">
        <div class="section-head"><Zap :size="14" /> 电价变化趋势</div>
        <div ref="priceChartRef" class="chart-box"></div>
      </section>

      <section class="detail-section">
        <div class="section-head"><Activity :size="14" /> 空闲率趋势</div>
        <div ref="historyChartRef" class="chart-box"></div>
        <div class="history-meta"><Clock3 :size="11" /> 最近一次采集 {{ fmtTime(detail.receivedAt) }}</div>
      </section>

      <section class="detail-section">
        <div class="section-head"><BatteryCharging :size="14" /> 电桩状态</div>
        <div class="pile-type-summary">
          <div>
            <span>快充</span>
            <strong>{{ detail.fastTotal }}</strong>
            <small>空闲 {{ detail.fastIdle }} · 忙碌 {{ detail.fastBusy }}</small>
          </div>
          <div>
            <span>超充</span>
            <strong>{{ detail.superTotal }}</strong>
            <small>空闲 {{ detail.superIdle }} · 忙碌 {{ detail.superBusy }}</small>
          </div>
          <div>
            <span>慢充</span>
            <strong>{{ detail.slowTotal }}</strong>
            <small>空闲 {{ detail.slowIdle }} · 忙碌 {{ detail.slowBusy }}</small>
          </div>
        </div>
        <div class="pile-summary">
          <span>空闲 <strong>{{ detail.pileIdle }}</strong></span>
          <span>忙碌 <strong>{{ detail.pileBusy }}</strong></span>
          <span v-if="detail.pileUnknown > 0">未知 <strong>{{ detail.pileUnknown }}</strong></span>
          <span>总数 <strong>{{ detail.pileTotal }}</strong></span>
        </div>
        <div v-if="detail.piles.length" class="pile-list">
          <div v-for="pile in detail.piles" :key="pile.deviceId" class="pile-row">
            <div>
              <strong>{{ pile.deviceId }}</strong>
              <span>{{ pileGroup(pile) }} · {{ pile.ratedPower || '功率未知' }} · {{ pile.ratedCurrent || '电流未知' }}</span>
            </div>
          </div>
        </div>
        <div v-else class="mini-empty">该站点本轮没有逐桩明细</div>
      </section>
    </template>
  </aside>
</template>
