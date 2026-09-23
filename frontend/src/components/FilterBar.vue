<script setup lang="ts">
import { onBeforeUnmount, reactive, watch } from 'vue'
import { Filter, RotateCcw, Search } from 'lucide-vue-next'
import { availabilityLabels } from '../insights'
import type { Meta, StationFilters } from '../types'

const props = defineProps<{
  filters: StationFilters
  meta: Meta | null
  total: number
}>()

const emit = defineEmits<{
  update: [filters: StationFilters]
  reset: []
}>()

const draft = reactive<StationFilters>({ ...props.filters })

const SEARCH_DEBOUNCE_MS = 300
let searchTimer: ReturnType<typeof setTimeout> | undefined
let searchComposing = false

function cancelSearch() {
  if (searchTimer !== undefined) clearTimeout(searchTimer)
  searchTimer = undefined
}

watch(() => props.filters, value => {
  cancelSearch()
  Object.assign(draft, value)
}, { deep: true })

function apply() {
  cancelSearch()
  emit('update', { ...draft, page: 1 })
}

function choose<K extends 'priceBand' | 'tou' | 'piles' | 'availability'>(key: K, value: StationFilters[K]) {
  draft[key] = value
  apply()
}

function scheduleSearch() {
  cancelSearch()
  if (!searchComposing) searchTimer = setTimeout(apply, SEARCH_DEBOUNCE_MS)
}

function beginSearchComposition() {
  searchComposing = true
  cancelSearch()
}

function endSearchComposition(event: CompositionEvent) {
  searchComposing = false
  draft.keyword = (event.target as HTMLInputElement).value
  scheduleSearch()
}

function searchOnEnter(event: KeyboardEvent) {
  if (!searchComposing && !event.isComposing) apply()
}

function reset() {
  cancelSearch()
  emit('reset')
}

onBeforeUnmount(cancelSearch)
</script>

<template>
  <aside class="filter-panel">
    <div class="panel-heading">
      <div><Filter :size="15" /> 筛选条件</div>
      <button class="icon-button" title="重置筛选" @click="reset"><RotateCcw :size="14" /></button>
    </div>

    <label class="search-field">
      <Search :size="15" />
      <input
        v-model="draft.keyword"
        placeholder="站点、POI 或城市"
        aria-label="搜索站点、POI 或城市"
        @input="scheduleSearch"
        @compositionstart="beginSearchComposition"
        @compositionend="endSearchComposition"
        @keyup.enter="searchOnEnter"
      />
    </label>

    <label>
      <span>城市</span>
      <select v-model="draft.city" @change="apply">
        <option value="">全部城市</option>
        <option v-for="item in meta?.cities" :key="item.name" :value="item.name">{{ item.name }} · {{ item.count }}</option>
      </select>
    </label>

    <label>
      <span>运营商</span>
      <select v-model="draft.operator" @change="apply">
        <option value="">全部运营商</option>
        <option v-for="item in meta?.operators" :key="item.name" :value="item.name">{{ item.name }} · {{ item.count }}</option>
      </select>
    </label>

    <div class="filter-group">
      <span>电价水平</span>
      <div class="segmented">
        <button :class="{ active: draft.priceBand === '' }" @click="choose('priceBand', '')">全部</button>
        <button :class="{ active: draft.priceBand === 'cheap' }" @click="choose('priceBand', 'cheap')">低价</button>
        <button :class="{ active: draft.priceBand === 'mid' }" @click="choose('priceBand', 'mid')">中位</button>
        <button :class="{ active: draft.priceBand === 'expensive' }" @click="choose('priceBand', 'expensive')">高价</button>
      </div>
    </div>

    <div class="filter-group">
      <span>分时费率</span>
      <div class="segmented vertical">
        <button :class="{ active: draft.tou === '' }" @click="choose('tou', '')">全部站点</button>
        <button :class="{ active: draft.tou === 'has' }" @click="choose('tou', 'has')">有多时段</button>
        <button :class="{ active: draft.tou === 'flat' }" @click="choose('tou', 'flat')">全天统一价</button>
        <button :class="{ active: draft.tou === 'none' }" @click="choose('tou', 'none')">无分时数据</button>
      </div>
    </div>

    <div class="filter-group">
      <span>电桩明细</span>
      <div class="segmented">
        <button :class="{ active: draft.piles === '' }" @click="choose('piles', '')">全部</button>
        <button :class="{ active: draft.piles === 'with' }" @click="choose('piles', 'with')">有</button>
        <button :class="{ active: draft.piles === 'without' }" @click="choose('piles', 'without')">无</button>
      </div>
    </div>

    <label>
      <span>排序</span>
      <select v-model="draft.sort" @change="apply">
        <option value="idleDesc">空闲率从高到低</option>
        <option value="updated">最近采集</option>
        <option value="priceAsc">电价从低到高</option>
        <option value="priceDesc">电价从高到低</option>
        <option value="periods">分时段数最多</option>
        <option value="name">站点名称</option>
      </select>
    </label>

    <button
      v-if="filters.availability"
      class="active-availability-filter"
      title="清除站点状态筛选"
      @click="choose('availability', '')"
    >
      站点状态：{{ availabilityLabels[filters.availability] }} <span aria-hidden="true">×</span>
    </button>
    <div class="filter-foot"><span class="filter-auto-hint">筛选后自动更新</span>{{ total }} 条匹配结果</div>
  </aside>
</template>
