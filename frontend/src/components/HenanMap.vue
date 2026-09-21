<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import * as echarts from 'echarts/core'
import { MapChart, ScatterChart } from 'echarts/charts'
import { GeoComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { CircleDollarSign, MapPinned, TimerReset, X } from 'lucide-vue-next'
import henanGeoJSON from '../assets/henan.json'
import type { HistoryPoint, MapStation, StationDetail } from '../types'
import StationDetailPanel from './StationDetail.vue'

echarts.use([MapChart, ScatterChart, GeoComponent, TooltipComponent, CanvasRenderer])
echarts.registerMap('henan', henanGeoJSON as never)

const props = defineProps<{
  visible: boolean
  items: MapStation[]
  loading: boolean
  updatedAt: number
  total: number
  detail: StationDetail | null
  history: HistoryPoint[]
  detailLoading: boolean
}>()

const emit = defineEmits<{
  close: []
  select: [sourceKey: string]
  clearDetail: []
}>()

const mapEl = ref<HTMLDivElement | null>(null)
let chart: echarts.EChartsType | null = null
let resizeObserver: ResizeObserver | null = null

const coordinateCount = computed(() => props.items.length)
const missingCoordinates = computed(() => Math.max(0, props.total - props.items.length))
const pricedCount = computed(() => props.items.filter(item => item.currentPrice > 0).length)
const availableCount = computed(() => props.items.filter(item => item.pileIdle > 0).length)
const averagePrice = computed(() => {
  const priced = props.items.filter(item => item.currentPrice > 0)
  if (!priced.length) return 0
  return priced.reduce((sum, item) => sum + item.currentPrice, 0) / priced.length
})
const topCheap = computed(() =>
  [...props.items]
    .filter(item => item.currentPrice > 0)
    .sort((a, b) => a.currentPrice - b.currentPrice)
    .slice(0, 20),
)

function escapeHtml(value: string) {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#039;')
}

function fmtTime(epoch: number) {
  if (!epoch) return '--'
  return new Date(epoch * 1000).toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  })
}

function fmtPrice(value: number) {
  return value > 0 ? `${value.toFixed(2)} 元/kWh` : '暂无电价'
}

function buildOption() {
  const data = props.items.map(item => ({
    name: item.name,
    value: [
      item.longitude,
      item.latitude,
      item.currentPrice > 0 ? item.currentPrice : null,
      item.idleRate,
      item.pileIdle,
      item.pileTotal,
      item.city,
      item.district,
      item.address,
    ],
    sourceKey: item.sourceKey,
    currentPriceText: item.currentPriceText,
    operator: item.operator,
    itemStyle: {
      color: item.priceLevel === 'cheap'
        ? '#31d17d'
        : item.priceLevel === 'expensive'
          ? '#f16f61'
          : item.priceLevel === 'mid'
            ? '#f0b43d'
            : '#52675f',
      borderColor: '#dceae5',
      borderWidth: 1,
      opacity: 0.9,
    },
  }))

  return {
    animationDuration: 500,
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'item',
      backgroundColor: '#0c1513',
      borderColor: '#2a463d',
      borderWidth: 1,
      padding: 10,
      textStyle: { color: '#dceae5', fontSize: 11 },
      formatter: (params: { data?: { name?: string; currentPriceText?: string; value?: unknown[]; operator?: string } }) => {
        const item = params.data
        if (!item?.value) return escapeHtml(item?.name || '充电站')
        const [, , price, idleRate, pileIdle, pileTotal, city, district, address] = item.value
        return [
          `<strong>${escapeHtml(item.name || '')}</strong>`,
          `${escapeHtml(String(city || ''))} ${escapeHtml(String(district || ''))}`,
          `当前电价：${price ? `${Number(price).toFixed(2)} 元/kWh` : '暂无'}`,
          `空闲桩：${pileIdle ?? 0} / ${pileTotal ?? 0}（${Number(idleRate ?? 0).toFixed(0)}%）`,
          item.operator ? `运营商：${escapeHtml(item.operator)}` : '',
          address ? `<span style="color:#748d84">${escapeHtml(String(address))}</span>` : '',
          '<span style="color:#31d17d">点击查看站点详情</span>',
        ]
          .filter(Boolean)
          .join('<br/>')
      },
    },
    geo: {
      map: 'henan',
      roam: true,
      zoom: 1.12,
      center: [113.4, 34.1],
      itemStyle: {
        areaColor: '#0e1c18',
        borderColor: '#31574a',
        borderWidth: 1,
      },
      label: { show: true, color: '#5d7a70', fontSize: 9 },
      emphasis: {
        label: { color: '#dceae5', fontSize: 10 },
        itemStyle: { areaColor: '#173129' },
      },
      select: { disabled: true },
    },
    series: [
      {
        name: '充电站',
        type: 'scatter',
        coordinateSystem: 'geo',
        data,
        symbol: 'circle',
        symbolSize: 6,
        itemStyle: {
          borderColor: '#dceae5',
          borderWidth: 0.5,
          opacity: 0.78,
        },
        emphasis: {
          scale: 1.8,
          itemStyle: { opacity: 1 },
        },
      },
    ],
  } as echarts.EChartsCoreOption
}

function render() {
  if (!props.visible || !mapEl.value) return
  if (chart && chart.getDom() !== mapEl.value) {
    chart.dispose()
    chart = null
  }
  chart ??= echarts.init(mapEl.value)
  chart.setOption(buildOption(), true)
  chart.resize()
  chart.off('click')
  chart.on('click', (params: unknown) => {
    const sourceKey = (params as { data?: { sourceKey?: string } | null }).data?.sourceKey
    if (sourceKey) emit('select', sourceKey)
  })
}

async function showMap() {
  await nextTick()
  render()
  if (typeof ResizeObserver !== 'undefined' && mapEl.value) {
    resizeObserver ??= new ResizeObserver(() => chart?.resize())
    resizeObserver.disconnect()
    resizeObserver.observe(mapEl.value)
  }
}

watch(() => [props.visible, props.items.length, props.updatedAt], () => {
  if (props.visible) void showMap()
})

onMounted(() => {
  if (props.visible) void showMap()
})

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  chart?.dispose()
  chart = null
})
</script>

<template>
  <Teleport to="body">
    <div v-if="visible" class="map-overlay" @keydown.esc="emit('close')" @click.self="emit('close')">
      <section class="map-panel">
        <header class="map-panel-head">
          <div>
            <span class="map-kicker"><MapPinned :size="12" /> HENAN · CHARGING NETWORK</span>
            <h2>河南重卡充电站地图总览</h2>
            <p>点颜色代表电价高低；点击站点可查看完整详情。</p>
            <div class="map-legend">
              <span><i class="dot cheap"></i>低价</span>
              <span><i class="dot mid"></i>中位</span>
              <span><i class="dot expensive"></i>高价</span>
              <span><i class="dot missing"></i>无电价</span>
            </div>
          </div>
          <div class="map-head-actions">
            <span class="map-updated"><TimerReset :size="12" /> 最近入库 {{ fmtTime(updatedAt) }}</span>
            <button class="icon-button" title="关闭地图总览" @click="emit('close')"><X :size="17" /></button>
          </div>
        </header>

        <div class="map-body">
          <div ref="mapEl" class="henan-map"></div>
          <div v-if="loading" class="map-loading">正在加载全省站点...</div>
          <aside class="map-side">
            <div class="map-stat-grid">
              <div>
                <span>地图覆盖</span>
                <strong>{{ coordinateCount }}<em>/{{ total }}</em></strong>
                <small v-if="missingCoordinates">缺坐标 {{ missingCoordinates }}</small>
              </div>
              <div><span>有效电价</span><strong>{{ pricedCount }}</strong></div>
              <div><span>有空闲桩</span><strong>{{ availableCount }}</strong></div>
              <div><span>平均电价</span><strong>{{ averagePrice.toFixed(2) }}<em>元</em></strong></div>
            </div>
            <div class="map-detail-wrap">
              <StationDetailPanel
                v-if="detail"
                :detail="detail"
                :history="history"
                :loading="detailLoading"
                @close="emit('clearDetail')"
              />
              <div v-else-if="detailLoading" class="map-detail-loading">正在加载站点详情...</div>
              <div v-else class="map-list">
                <div class="map-list-head"><CircleDollarSign :size="13" /> 低价站点 TOP {{ topCheap.length }}</div>
                <button
                  v-for="item in topCheap"
                  :key="item.sourceKey"
                  class="map-list-row"
                  @click="emit('select', item.sourceKey)"
                >
                  <span>{{ item.name }}<small>{{ item.city }} {{ item.district }}</small></span>
                  <strong>{{ fmtPrice(item.currentPrice) }}</strong>
                </button>
                <div v-if="!topCheap.length" class="map-list-empty">暂无有效电价数据</div>
              </div>
            </div>
          </aside>
        </div>
      </section>
    </div>
  </Teleport>
</template>
