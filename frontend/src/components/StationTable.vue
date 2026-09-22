<script setup lang="ts">
import { computed } from 'vue'
import { ChevronLeft, ChevronRight, MapPin, Zap } from 'lucide-vue-next'
import type { Station } from '../types'

const props = defineProps<{
  items: Station[]
  loading: boolean
  total: number
  page: number
  pageSize: number
  selectedKey: string
  viewMode: 'list' | 'card'
  selectedCity: string
}>()

const emit = defineEmits<{
  select: [station: Station]
  page: [page: number]
  'update:viewMode': [mode: 'list' | 'card']
}>()

function pages() {
  return Math.max(1, Math.ceil(props.total / props.pageSize))
}

function priceClass(station: Station) {
  return `price-${station.priceLevel}`
}

function fmtTime(epoch: number) {
  if (!epoch) return '--'
  return new Date(epoch * 1000).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false })
}

function loadClass(station: Station) {
  if (!station.hasPileDetails || station.pileTotal <= 0) return 'unknown'
  if (station.idleRate >= 50) return 'idle'
  if (station.idleRate >= 20) return 'moderate'
  return 'full'
}

function loadText(station: Station) {
  if (!station.hasPileDetails || station.pileTotal <= 0) return '无桩详情'
  if (station.idleRate >= 50) return '空闲充足'
  if (station.idleRate >= 20) return '占用适中'
  return '接近满载'
}

function summarizeStations(stations: Station[]) {
  const summary = { idle: 0, moderate: 0, full: 0, unknown: 0 }
  for (const station of stations) {
    summary[loadClass(station) as keyof typeof summary] += 1
  }
  return summary
}

function hashKey(value: string) {
  let hash = 2166136261
  for (let index = 0; index < value.length; index += 1) {
    hash ^= value.charCodeAt(index)
    hash = Math.imul(hash, 16777619)
  }
  return hash >>> 0
}

const shuffleSalt = Math.random().toString(36).slice(2)
const groupedCards = computed(() => {
  const stations = props.selectedCity
    ? [...props.items]
    : [...props.items].sort((left, right) =>
        hashKey(left.sourceKey + shuffleSalt) - hashKey(right.sourceKey + shuffleSalt),
      )
  return [{
    city: props.selectedCity,
    stations,
    summary: summarizeStations(stations),
  }]
})
</script>

<template>
  <section class="table-panel">
    <div class="table-toolbar">
      <div>
        <strong>站点实时总览</strong>
        <span>{{ total }} 个站点 · 相对电价色阶 · 空闲率独立进度</span>
      </div>
      <div class="toolbar-right">
        <nav v-if="viewMode === 'card'" class="legend">
          <span><i class="dot status-idle"></i>空闲</span>
          <span><i class="dot status-moderate"></i>适中</span>
          <span><i class="dot status-full"></i>满载</span>
          <span><i class="dot status-unknown"></i>无数据</span>
        </nav>
        <div v-else class="legend">
          <span><i class="dot cheap"></i>低价</span>
          <span><i class="dot mid"></i>中位</span>
          <span><i class="dot expensive"></i>高价</span>
          <span><i class="dot missing"></i>缺失</span>
        </div>
        <div class="view-tabs" role="tablist" aria-label="站点展示方式">
          <button
            role="tab"
            :aria-selected="viewMode === 'list'"
            :class="{ active: viewMode === 'list' }"
            @click="emit('update:viewMode', 'list')"
          >列表</button>
          <button
            role="tab"
            :aria-selected="viewMode === 'card'"
            :class="{ active: viewMode === 'card' }"
            @click="emit('update:viewMode', 'card')"
          >卡片</button>
        </div>
      </div>
    </div>

    <div v-if="viewMode === 'list'" class="table-scroll">
      <table class="station-table">
        <thead>
          <tr>
            <th>站点</th>
            <th>城市 / 运营商</th>
            <th>当前电价</th>
            <th>分时费率</th>
            <th>快 / 超 / 慢</th>
            <th>空闲</th>
            <th>采集时间</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="loading" v-for="index in 8" :key="index" class="skeleton-row">
            <td colspan="7"><span></span></td>
          </tr>
          <tr
            v-else
            v-for="station in items"
            :key="station.sourceKey"
            :class="{ selected: station.sourceKey === selectedKey }"
            @click="emit('select', station)"
          >
            <td>
              <div class="station-name" :title="station.matchedName">{{ station.matchedName || station.requestedName }}</div>
              <div class="station-sub"><MapPin :size="11" /> {{ station.district || station.sourceStationId || '未知区域' }}</div>
            </td>
            <td>
              <div>{{ station.city || '--' }}</div>
              <div class="station-sub">{{ station.operator || '未知运营商' }}</div>
            </td>
            <td>
              <div class="price-cell" :class="priceClass(station)">
                <strong>{{ station.currentPriceText || '--' }}</strong>
                <span v-if="station.currentPriceText">元/kWh</span>
              </div>
            </td>
            <td>
              <span v-if="station.hasTou" class="rate-badge tou">多时段 {{ station.pricePeriodCount }}</span>
              <span v-else-if="station.flatOnly" class="rate-badge flat">全天统一</span>
              <span v-else class="rate-badge missing">无数据</span>
            </td>
            <td>
              <div class="pile-triplet">
                <span><Zap :size="11" />快 {{ station.fastTotal }}</span>
                <span class="super">超 {{ station.superTotal }}</span>
                <span class="slow">慢 {{ station.slowTotal }}</span>
              </div>
            </td>
            <td>
              <template v-if="station.hasPileDetails && station.pileTotal > 0">
                <div class="idle-cell">
                  <div class="idle-track"><i :style="{ width: `${Math.min(100, station.idleRate)}%` }"></i></div>
                  <span>{{ station.idleRate.toFixed(0) }}%</span>
                </div>
                <div class="station-sub">{{ station.pileIdle }} / {{ station.pileTotal }} 根空闲</div>
              </template>
              <template v-else>
                <div class="idle-missing">暂无数据</div>
                <div class="station-sub">未采集到桩详情</div>
              </template>
            </td>
            <td class="time-cell">{{ fmtTime(station.receivedAt) }}</td>
          </tr>
          <tr v-if="!loading && !items.length">
            <td colspan="7" class="empty-state">没有符合当前条件的站点</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-else class="card-scroll">
      <div v-if="loading" class="station-card-grid">
        <div v-for="index in 12" :key="index" class="station-card skeleton-card"><span></span></div>
      </div>

      <template v-else-if="groupedCards.length">
        <section v-for="group in groupedCards" :key="group.city" class="city-group">
          <header v-if="group.city" class="city-group-head">
            <div>
              <strong>{{ group.city }}</strong>
              <span>{{ group.stations.length }} 站</span>
            </div>
            <div class="city-summary">
              <span><i class="dot status-idle"></i>{{ group.summary.idle }}</span>
              <span><i class="dot status-moderate"></i>{{ group.summary.moderate }}</span>
              <span><i class="dot status-full"></i>{{ group.summary.full }}</span>
              <span><i class="dot status-unknown"></i>{{ group.summary.unknown }}</span>
            </div>
          </header>

          <div class="station-card-grid">
            <button
              v-for="station in group.stations"
              :key="station.sourceKey"
              class="station-card"
              :class="[loadClass(station), { selected: station.sourceKey === selectedKey }]"
              type="button"
              :title="station.matchedName || station.requestedName"
              @click="emit('select', station)"
            >
              <span class="card-status">{{ loadText(station) }}</span>
              <span class="card-name">{{ station.matchedName || station.requestedName }}</span>
              <span class="card-location">
                <MapPin :size="10" />
                {{ station.district || station.sourceStationId || '未知区县' }} · {{ station.operator || '未知运营商' }}
              </span>

              <span class="card-price">
                <strong>{{ station.currentPriceText || '--' }}</strong>
                <small>{{ station.currentPriceText ? '元/kWh' : '暂无电价' }}</small>
              </span>

              <span class="card-rate">
                <span v-if="station.hasTou">分时 {{ station.pricePeriodCount }}</span>
                <span v-else-if="station.flatOnly">全天统一</span>
                <span v-else>无分时</span>
              </span>

              <span class="card-pile-type">
                <span><Zap :size="10" />快 {{ station.fastTotal }}</span>
                <span class="super">超 {{ station.superTotal }}</span>
                <span class="slow">慢 {{ station.slowTotal }}</span>
              </span>

              <span class="card-load">
                <template v-if="station.hasPileDetails && station.pileTotal > 0">
                  <i class="load-track"><b :style="{ width: `${Math.min(100, station.idleRate)}%` }"></b></i>
                  <small>{{ station.pileIdle }}/{{ station.pileTotal }} 空闲</small>
                </template>
                <template v-else>
                  <i class="load-track empty"></i>
                  <small>暂无数据</small>
                </template>
              </span>

              <span class="card-time">{{ fmtTime(station.receivedAt) }}</span>
            </button>
          </div>
        </section>
      </template>

      <div v-else class="empty-state card-empty">没有符合当前条件的站点</div>
    </div>

    <div v-if="viewMode === 'list'" class="pagination">
      <span>第 {{ page }} / {{ pages() }} 页 · 每页 {{ pageSize }} 站</span>
      <div>
        <button class="icon-button" :disabled="page <= 1" @click="emit('page', page - 1)"><ChevronLeft :size="15" /></button>
        <button class="icon-button" :disabled="page >= pages()" @click="emit('page', page + 1)"><ChevronRight :size="15" /></button>
      </div>
    </div>
    <div v-else class="pagination">
      <span>已加载全部 {{ total }} 个站点 · 按地市横向浏览</span>
    </div>
  </section>
</template>
