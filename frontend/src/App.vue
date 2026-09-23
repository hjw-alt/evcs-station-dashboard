<script setup lang="ts">
import { onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { Activity, MapPinned, RefreshCw, Server } from 'lucide-vue-next'
import { fetchMapStations, fetchMeta, fetchOverview, fetchStationDetail, fetchStationHistory, fetchStations } from './api'
import type { Availability, StationSummary, HistoryPoint, MapStation, Meta, Overview, Station, StationDetail, StationFilters } from './types'
import KpiStrip from './components/KpiStrip.vue'
import FilterBar from './components/FilterBar.vue'
import HenanMap from './components/HenanMap.vue'
import StationTable from './components/StationTable.vue'
import StationDetailPanel from './components/StationDetail.vue'
import StationInsights from './components/StationInsights.vue'

const REFRESH_INTERVAL_MS = 5 * 60_000
const viewMode = ref<'list' | 'card'>('card')
const overview = ref<Overview | null>(null)
const meta = ref<Meta | null>(null)
const stations = ref<Station[]>([])
const total = ref(0)
const summary = ref<StationSummary | null>(null)
const summaryError = ref('')
let stationRequestId = 0
let detailRequestId = 0
const loading = ref(true)
const detailLoading = ref(false)
const selectedKey = ref('')
const detail = ref<StationDetail | null>(null)
const history = ref<HistoryPoint[]>([])
const error = ref('')
const refreshedAt = ref(0)
const mapVisible = ref(false)
const mapLoading = ref(false)
const mapStations = ref<MapStation[]>([])
const mapUpdatedAt = ref(0)
let refreshTimer: number | undefined
let previousBodyOverflow = ''

const filters = reactive<StationFilters>({
  availability: '',
  keyword: '',
  city: '郑州市',
  operator: '',
  tou: '',
  priceBand: '',
  piles: '',
  sort: 'idleDesc',
  page: 1,
  pageSize: 3000,
})

async function loadOverview() {
  overview.value = await fetchOverview()
  refreshedAt.value = Math.floor(Date.now() / 1000)
}

async function loadStations(silent = false) {
  const requestId = ++stationRequestId
  const requestedFilters = { ...filters }
  if (!silent) {
    loading.value = true
    summary.value = null
  }
  summaryError.value = ''
  try {
    const response = await fetchStations(requestedFilters)
    if (requestId !== stationRequestId) return
    stations.value = response.items || []
    total.value = response.total
    summary.value = response.summary ?? null
    if (!response.summary) {
      summaryError.value = '站点接口缺少概览数据，请更新并重启后端服务后重试。'
    }
  } catch (err) {
    if (requestId !== stationRequestId) return
    const message = err instanceof Error ? err.message : String(err)
    error.value = message
    summaryError.value = message
    summary.value = null
  } finally {
    if (requestId === stationRequestId) loading.value = false
  }
}

async function loadAll() {
  error.value = ''
  try {
    await Promise.all([loadOverview(), loadStations()])
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

async function selectBySourceKey(sourceKey: string) {
  const requestId = ++detailRequestId
  selectedKey.value = sourceKey
  detail.value = null
  history.value = []
  detailLoading.value = true
  try {
    const [stationDetail, stationHistory] = await Promise.all([
      fetchStationDetail(sourceKey),
      fetchStationHistory(sourceKey),
    ])
    if (requestId !== detailRequestId) return
    detail.value = stationDetail
    history.value = stationHistory.items || []
  } catch (err) {
    if (requestId !== detailRequestId) return
    error.value = err instanceof Error ? err.message : String(err)
    closeDetail()
  } finally {
    if (requestId === detailRequestId) detailLoading.value = false
  }
}

async function selectStation(station: Station) {
  await selectBySourceKey(station.sourceKey)
}

async function openMap() {
  if (!mapVisible.value) {
    previousBodyOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
  }
  mapVisible.value = true
  mapLoading.value = true
  try {
    const response = await fetchMapStations()
    mapStations.value = response.items || []
    mapUpdatedAt.value = response.updatedAt
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  } finally {
    mapLoading.value = false
  }
}

function closeMap() {
  mapVisible.value = false
  document.body.style.overflow = previousBodyOverflow
}

async function selectMapStation(sourceKey: string) {
  await selectBySourceKey(sourceKey)
}

function clearMapDetail() {
  closeDetail()
}

function applyFilters(next: StationFilters) {
  Object.assign(filters, next)
  closeDetail()
  void loadStations()
}

function setViewMode(mode: 'list' | 'card') {
  if (viewMode.value === mode) return
  viewMode.value = mode
  filters.pageSize = mode === 'card' ? 3000 : 25
  filters.page = 1
  void loadStations()
}

function resetFilters() {
  closeDetail()
  Object.assign(filters, {
    keyword: '', city: '', operator: '', tou: '', priceBand: '', piles: '', availability: '', sort: 'idleDesc', page: 1,
  })
  void loadStations()
}

function changePage(page: number) {
  filters.page = page
  void loadStations()
}

function closeDetail() {
  ++detailRequestId
  detailLoading.value = false
  selectedKey.value = ''
  detail.value = null
  history.value = []
}

function filterAvailability(availability: Availability | '') {
  applyFilters({ ...filters, availability, page: 1 })
}

function fmtTime(epoch: number) {
  if (!epoch) return '--'
  return new Date(epoch * 1000).toLocaleTimeString('zh-CN', { hour12: false })
}

onMounted(async () => {
  await loadAll()
  meta.value = await fetchMeta().catch(() => null)
  const params = new URLSearchParams(window.location.search)
  const deepLinkKey = params.get('station')
  if (deepLinkKey) await selectBySourceKey(deepLinkKey)
  if (params.get('map') === '1') await openMap()
  refreshTimer = window.setInterval(() => {
    void loadOverview().catch(() => undefined)
    void loadStations(true).catch(() => undefined)
  }, REFRESH_INTERVAL_MS)
})

onBeforeUnmount(() => {
  ++stationRequestId
  ++detailRequestId
  if (refreshTimer) window.clearInterval(refreshTimer)
  document.body.style.overflow = previousBodyOverflow
})
</script>

<template>
  <div class="app-shell">
    <header class="app-header">
      <div class="brand-block">
        <div class="brand-mark"><Activity :size="18" /></div>
        <div>
          <strong>重卡充电站运营监控台</strong>
          <span>PRICE · TOU · PILE · SNAPSHOT</span>
        </div>
      </div>
      <nav class="header-nav">
        <button class="map-entry" :class="{ active: mapVisible }" @click="openMap">
          <MapPinned :size="14" />
          河南地图总览
        </button>
      </nav>
      <div class="header-status">
        <span class="status-line"><Server :size="13" /> 最新刷新 {{ fmtTime(refreshedAt) }}</span>
        <button class="icon-button" title="立即刷新" @click="loadAll"><RefreshCw :size="14" /></button>
      </div>
    </header>

    <HenanMap
      :visible="mapVisible"
      :items="mapStations"
      :loading="mapLoading"
      :updated-at="mapUpdatedAt"
      :total="overview?.total ?? 0"
      :detail="detail"
      :history="history"
      :detail-loading="detailLoading"
      @close="closeMap"
      @select="selectMapStation"
      @clear-detail="clearMapDetail"
    />

    <div v-if="error" class="error-bar">{{ error }}</div>

    <KpiStrip :overview="overview" />

    <main class="dashboard-grid">
      <FilterBar :filters="filters" :meta="meta" :total="total" @update="applyFilters" @reset="resetFilters" />
      <StationTable
        :items="stations"
        :loading="loading"
        :total="total"
        :page="filters.page"
        :page-size="filters.pageSize"
        :selected-key="selectedKey"
        :view-mode="viewMode"
        :selected-city="filters.city"
        @select="selectStation"
        @page="changePage"
        @update:view-mode="setViewMode"
      />
      <StationInsights
        v-if="!selectedKey"
        :summary="summary"
        :filters="filters"
        :loading="loading"
        :error="summaryError"
        @select="selectStation"
        @filter-availability="filterAvailability"
        @retry="loadAll"
      />
      <StationDetailPanel v-else :detail="detail" :history="history" :loading="detailLoading" @close="closeDetail" />
    </main>
  </div>
</template>
