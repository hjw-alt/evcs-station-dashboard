<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { MapPinned, TimerReset } from 'lucide-vue-next'
import { readChartTheme } from '../theme'
import henanCountiesGeoJSON from '../assets/henan-counties.json'
import type { MapStation } from '../types'
import * as echarts from 'echarts/core'
import { MapChart, ScatterChart } from 'echarts/charts'
import { GeoComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'

echarts.use([MapChart, ScatterChart, GeoComponent, TooltipComponent, CanvasRenderer])
echarts.registerMap('henan-counties', henanCountiesGeoJSON as never)

const props = defineProps<{
  items: MapStation[]
  loading: boolean
  updatedAt: number
}>()

const emit = defineEmits<{
  select: [sourceKey: string]
}>()

const mapEl = ref<HTMLDivElement | null>(null)
let chart: echarts.EChartsType | null = null
let resizeObserver: ResizeObserver | null = null

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

function buildOption() {
  const theme = readChartTheme()
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
        ? theme.green
        : item.priceLevel === 'expensive'
          ? theme.red
          : item.priceLevel === 'mid'
            ? theme.amber
            : theme.unknown,
      borderColor: theme.panel,
      borderWidth: 1,
      opacity: 0.9,
    },
  }))

  return {
    animationDuration: 500,
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'item',
      backgroundColor: theme.panel,
      borderColor: theme.lineStrong,
      borderWidth: 1,
      padding: 10,
      textStyle: { color: theme.text, fontSize: 11 },
      formatter: (params: { data?: { name?: string; value?: unknown[]; operator?: string } }) => {
        const item = params.data
        if (!item?.value) return escapeHtml(item?.name || '充电站')
        const [, , price, idleRate, pileIdle, pileTotal, city, district, address] = item.value
        return [
          `<strong>${escapeHtml(item.name || '')}</strong>`,
          `${escapeHtml(String(city || ''))} ${escapeHtml(String(district || ''))}`,
          `当前电价：${price ? `${Number(price).toFixed(2)} 元/kWh` : '暂无'}`,
          `空闲桩：${pileIdle ?? 0} / ${pileTotal ?? 0}（${Number(idleRate ?? 0).toFixed(0)}%）`,
          item.operator ? `运营商：${escapeHtml(item.operator)}` : '',
          address ? `<span style="color:${theme.muted}">${escapeHtml(String(address))}</span>` : '',
          `<span style="color:${theme.primary}">点击查看站点详情</span>`,
        ]
          .filter(Boolean)
          .join('<br/>')
      },
    },
    geo: {
      map: 'henan-counties',
      roam: true,
      zoom: 1.12,
      scaleLimit: { min: 1, max: 10 },
      center: [113.4, 34.1],
      itemStyle: {
        areaColor: theme.mapArea,
        borderColor: theme.lineStrong,
        borderWidth: 0.6,
      },
      label: {
        show: false,
        color: theme.muted,
        fontSize: 8,
      },
      emphasis: {
        label: { show: true, color: theme.text, fontSize: 9 },
        itemStyle: { areaColor: theme.mapHover },
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
          borderColor: theme.panel,
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
  if (!mapEl.value) return
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

watch(() => [props.items.length, props.updatedAt], () => void showMap())

onMounted(() => {
  void showMap()
})

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  chart?.dispose()
  chart = null
})
</script>

<template>
  <section class="map-panel">
    <div class="map-body">
      <div ref="mapEl" class="henan-map"></div>
      <div v-if="loading" class="map-loading">正在加载全省站点...</div>
      <aside class="map-hint" aria-label="地图说明">
        <h2><MapPinned :size="13" /> 河南重卡充电站地图总览</h2>
        <p>滚轮缩放 · 拖动平移 · 点击站点查看右侧详情</p>
        <div class="map-legend">
          <span><i class="dot cheap"></i>低价</span>
          <span><i class="dot mid"></i>中位</span>
          <span><i class="dot expensive"></i>高价</span>
          <span><i class="dot missing"></i>无电价</span>
        </div>
        <span class="map-updated"><TimerReset :size="12" /> 最近入库 {{ fmtTime(updatedAt) }}</span>
      </aside>
    </div>
  </section>
</template>
