<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { fetchMapStations, fetchOverview, fetchStationDetail, fetchStationHistory, fetchStations } from './api'
import type { StationFacets, StationSummary, HistoryPoint, MapStation, Overview, Station, StationDetail, StationFilters } from './types'
import KpiStrip from './components/KpiStrip.vue'
import FilterBar from './components/FilterBar.vue'
import StationTable from './components/StationTable.vue'
import StationDetailPanel from './components/StationDetail.vue'

const REFRESH_INTERVAL_MS = 5 * 60_000
const viewMode = ref<'list' | 'card' | 'map' | 'report'>('card')
const overview = ref<Overview | null>(null)
const facets = ref<StationFacets | null>(null)
const matchedKeys = ref<Set<string>>(new Set())
const filteredMapStations = computed(() => mapStations.value.filter(station => matchedKeys.value.has(station.sourceKey)))
const stations = ref<Station[]>([])
const total = ref(0)
const summary = ref<StationSummary | null>(null)

let stationRequestId = 0
let detailRequestId = 0
const loading = ref(true)
const detailLoading = ref(false)
const selectedKey = ref('')
const detailModalRef = ref<HTMLDivElement | null>(null)
let detailTrigger: HTMLElement | null = null
let previousBodyOverflow: string | null = null
const detail = ref<StationDetail | null>(null)
const history = ref<HistoryPoint[]>([])
const error = ref('')
const refreshedAt = ref(0)
const mapLoading = ref(false)
const mapStations = ref<MapStation[]>([])
const mapUpdatedAt = ref(0)
let mapRequestId = 0
let refreshTimer: number | undefined

const filters = reactive<StationFilters>({
  availability: '',
  keyword: '',
  city: '',
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
  try {
    const response = await fetchStations(requestedFilters)
    if (requestId !== stationRequestId) return
    stations.value = response.items || []
    total.value = response.total
    summary.value = response.summary ?? null
    facets.value = response.facets ?? null
    matchedKeys.value = new Set(response.matchedKeys ?? [])
    if (!response.facets || !response.matchedKeys) {
      error.value = '站点接口缺少筛选计数，请更新并重启后端服务。'
    }
    if (!response.summary) {
      error.value = '站点接口缺少概览数据，请更新并重启后端服务后重试。'
    }
  } catch (err) {
    if (requestId !== stationRequestId) return
    const message = err instanceof Error ? err.message : String(err)
    error.value = message
    facets.value = null
    matchedKeys.value = new Set()
    summary.value = null
  } finally {
    if (requestId === stationRequestId) loading.value = false
  }
}

async function loadAll() {
  error.value = ''
  try {
    await Promise.all([loadOverview(), loadStations(), ...(viewMode.value === 'map' ? [loadMap()] : [])])
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

async function selectBySourceKey(sourceKey: string) {
  if (!selectedKey.value) {
    detailTrigger = document.activeElement instanceof HTMLElement ? document.activeElement : null
  }
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

async function loadMap(silent = false) {
  const requestId = ++mapRequestId
  if (!silent) mapLoading.value = true
  try {
    const response = await fetchMapStations()
    if (requestId !== mapRequestId) return
    mapStations.value = response.items || []
    mapUpdatedAt.value = response.updatedAt
  } catch (err) {
    if (requestId !== mapRequestId) return
    error.value = err instanceof Error ? err.message : String(err)
  } finally {
    if (requestId === mapRequestId) mapLoading.value = false
  }
}

async function selectMapStation(sourceKey: string) {
  await selectBySourceKey(sourceKey)
}

function applyFilters(next: StationFilters) {
  Object.assign(filters, next, { piles: '' })
  closeDetail()
  void loadStations()
}

function setViewMode(mode: 'list' | 'card' | 'map' | 'report') {
  if (viewMode.value === mode) {
    if (mode === 'map') void loadMap()
    return
  }
  viewMode.value = mode
  closeDetail()
  if (mode === 'map') {
    void loadMap()
    return
  }
  if (mode === 'report') return
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

function restoreBodyScroll() {
  if (previousBodyOverflow === null) return
  document.body.style.overflow = previousBodyOverflow
  previousBodyOverflow = null
}

watch(() => Boolean(selectedKey.value), (open) => {
  if (open) {
    previousBodyOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    detailModalRef.value?.querySelector<HTMLButtonElement>('.detail-close')?.focus({ preventScroll: true })
  } else {
    restoreBodyScroll()
    if (detailTrigger?.isConnected) detailTrigger.focus({ preventScroll: true })
    detailTrigger = null
  }
}, { flush: 'post' })

function handleGlobalKeydown(event: KeyboardEvent) {
  if (!selectedKey.value) return
  if (event.key === 'Escape') {
    event.preventDefault()
    closeDetail()
    return
  }
  if (event.key !== 'Tab' || !detailModalRef.value) return
  const focusable = Array.from(detailModalRef.value.querySelectorAll<HTMLElement>(
    'button:not([disabled]), a[href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
  )).filter(element => element.getClientRects().length > 0)
  const first = focusable[0]
  const last = focusable[focusable.length - 1]
  if (!first || !last) {
    event.preventDefault()
    detailModalRef.value.focus()
  } else if (event.shiftKey && (document.activeElement === first || document.activeElement === detailModalRef.value)) {
    event.preventDefault()
    last.focus()
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault()
    first.focus()
  }
}


onMounted(async () => {
  window.addEventListener('keydown', handleGlobalKeydown)
  await loadAll()
  const params = new URLSearchParams(window.location.search)
  const deepLinkKey = params.get('station')
  if (deepLinkKey) await selectBySourceKey(deepLinkKey)
  if (params.get('map') === '1') {
    viewMode.value = 'map'
    await loadMap()
  }
  refreshTimer = window.setInterval(() => {
    void loadOverview().catch(() => undefined)
    void loadStations(true).catch(() => undefined)
    if (viewMode.value === 'map') void loadMap(true).catch(() => undefined)
  }, REFRESH_INTERVAL_MS)
})

onBeforeUnmount(() => {
  ++stationRequestId
  ++detailRequestId
  ++mapRequestId
  if (refreshTimer) window.clearInterval(refreshTimer)
  window.removeEventListener('keydown', handleGlobalKeydown)
  restoreBodyScroll()
})
</script>

<template>
  <div class="app-shell" :inert="Boolean(selectedKey)">
    <KpiStrip :overview="overview" />

    <div v-if="error" class="error-bar" role="alert">{{ error }}</div>

    <FilterBar
      :filters="filters" :facets="facets" :total="total" :loading="loading" :refreshed-at="refreshedAt"
      @update="applyFilters" @reset="resetFilters" @refresh="loadAll"
    />

    <main class="dashboard-grid">
      <StationTable
        :items="stations"
        :loading="loading"
        :total="total"
        :page="filters.page"
        :page-size="filters.pageSize"
        :selected-key="selectedKey"
        :view-mode="viewMode"
        :selected-city="filters.city === '__unknown__' ? '未标注城市' : filters.city"
        :map-items="filteredMapStations"
        :map-loading="mapLoading || loading"
        :map-updated-at="mapUpdatedAt"
        :map-total="filteredMapStations.length"
        :overview="overview"
        :summary="summary"
        :filters="filters"
        @select="selectStation"
        @select-map="selectMapStation"
        @page="changePage"
        @update:view-mode="setViewMode"
      />
    </main>

    <Teleport to="body">
      <div
        v-if="selectedKey"
        ref="detailModalRef"
        class="detail-modal"
        tabindex="-1"
        role="dialog"
        aria-modal="true"
        aria-label="站点详情"
        @click.self="closeDetail"
      >
        <div class="detail-modal-card">
          <StationDetailPanel :detail="detail" :history="history" :loading="detailLoading" @close="closeDetail" />
        </div>
      </div>
    </Teleport>
  </div>
</template>
