<script setup lang="ts">
import { onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { Activity, MapPinned, RefreshCw, Server } from 'lucide-vue-next'
import { fetchMapStations, fetchMeta, fetchOverview, fetchStationDetail, fetchStationHistory, fetchStations } from './api'
import type { HistoryPoint, MapStation, Meta, Overview, Station, StationDetail, StationFilters } from './types'
import KpiStrip from './components/KpiStrip.vue'
import FilterBar from './components/FilterBar.vue'
import HenanMap from './components/HenanMap.vue'
import StationTable from './components/StationTable.vue'
import StationDetailPanel from './components/StationDetail.vue'

const overview = ref<Overview | null>(null)
const meta = ref<Meta | null>(null)
const stations = ref<Station[]>([])
const total = ref(0)
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
  keyword: '',
  city: '',
  operator: '',
  tou: '',
  priceBand: '',
  piles: '',
  sort: 'updated',
  page: 1,
  pageSize: 25,
})

async function loadOverview() {
  overview.value = await fetchOverview()
  refreshedAt.value = Math.floor(Date.now() / 1000)
}

async function loadStations() {
  loading.value = true
  try {
    const response = await fetchStations(filters)
    stations.value = response.items || []
    total.value = response.total
  } finally {
    loading.value = false
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
  selectedKey.value = sourceKey
  detailLoading.value = true
  try {
    const [stationDetail, stationHistory] = await Promise.all([
      fetchStationDetail(sourceKey),
      fetchStationHistory(sourceKey),
    ])
    detail.value = stationDetail
    history.value = stationHistory.items || []
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  } finally {
    detailLoading.value = false
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
  detail.value = null
  history.value = []
  selectedKey.value = ''
}

function applyFilters(next: StationFilters) {
  Object.assign(filters, next)
  detail.value = null
  selectedKey.value = ''
  void loadStations()
}

function resetFilters() {
  Object.assign(filters, {
    keyword: '', city: '', operator: '', tou: '', priceBand: '', piles: '', sort: 'updated', page: 1,
  })
  void loadStations()
}

function changePage(page: number) {
  filters.page = page
  void loadStations()
}

function closeDetail() {
  selectedKey.value = ''
  detail.value = null
  history.value = []
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
    void loadStations().catch(() => undefined)
  }, 60_000)
})

onBeforeUnmount(() => {
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
        @select="selectStation"
        @page="changePage"
      />
      <StationDetailPanel :detail="detail" :history="history" :loading="detailLoading" @close="closeDetail" />
    </main>
  </div>
</template>
