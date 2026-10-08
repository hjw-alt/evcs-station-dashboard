<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { ChevronDown, ChevronUp, RefreshCw, RotateCcw, Search } from 'lucide-vue-next'
import { mergeFilterOptions, readFilterOptions, saveFilterOptions } from '../filterOptions'
import type { StationFacetKey, StationFacets, StationFilters } from '../types'

const props = defineProps<{
  filters: StationFilters
  facets: StationFacets | null
  loading: boolean
  total: number
  refreshedAt?: number
}>()

const emit = defineEmits<{
  update: [filters: StationFilters]
  reset: []
  refresh: []
}>()

const draft = reactive<StationFilters>({ ...props.filters })

const optionNames = ref(readFilterOptions())
watch(() => props.facets, facets => {
  if (!facets) return
  optionNames.value = mergeFilterOptions(optionNames.value, facets)
  saveFilterOptions(optionNames.value)
}, { immediate: true, deep: true })

const expanded = reactive<Record<string, boolean>>({})
const rowLimit = 8
const fixedOptions: Partial<Record<StationFacetKey, [string, string][]>> = {
  availability: [['idle', '空闲'], ['moderate', '适中'], ['full', '满载'], ['unknown', '无数据']],
  priceBand: [['cheap', '低价'], ['mid', '中位价'], ['expensive', '高价'], ['missing', '无电价']],
}
const rowLabels: [StationFacetKey, string][] = [
  ['city', '所在城市'], ['operator', '运营商'], ['availability', '空闲状态'],
  ['priceBand', '电价水平'],
]
const rows = computed(() => rowLabels.map(([key, label]) => {
  const counts = props.facets?.[key]?.counts
  const dynamic = key === 'city' || key === 'operator'
  const names = dynamic ? [...optionNames.value[key]] : []
  // A selected option remains visible even when a search has no matching stations.
  if (!fixedOptions[key] && draft[key] && !names.includes(draft[key])) names.push(draft[key])
  const options = (fixedOptions[key] ?? names.map(name => [name, name === '__unknown__' ? (key === 'city' ? '未标注城市' : '未知运营商') : name])).map(([value, text]) => ({
    value: value!, label: text!, count: counts ? counts[value!] ?? 0 : null,
  }))
  let visible = expanded[key] ? options : options.slice(0, rowLimit)
  const selected = options.find(option => option.value === draft[key])
  if (selected && !visible.includes(selected)) visible = [...visible.slice(0, rowLimit - 1), selected]
  const placeholderCount = dynamic && !optionNames.value[key].length && props.loading ? Math.max(0, rowLimit - visible.length) : 0
  const emptyMessage = dynamic && !options.length && !props.loading
    ? (counts ? '暂无可选项' : '选项暂不可用，请刷新重试') : ''
  return { key, label, options: visible, optionCount: options.length, placeholderCount, emptyMessage }
}))

function countLabel(count: number | null | undefined) {
  return props.loading || count == null ? '—' : String(count)
}

const refreshTitle = computed(() => props.refreshedAt
  ? `刷新数据（上次刷新 ${new Date(props.refreshedAt * 1000).toLocaleTimeString('zh-CN', { hour12: false })}）`
  : '刷新数据')

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
  emit('update', { ...draft, piles: '', page: 1 })
}

function choose<K extends StationFacetKey>(key: K, value: StationFilters[K]) {
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
  <section class="filter-panel filter-panel-horizontal" aria-label="站点筛选">
    <div class="filter-rows">
      <div v-for="row in rows" :key="row.key" class="facet-row" :data-filter="row.key">
        <h2 :id="`filter-${row.key}-label`">{{ row.label }}</h2>
        <div class="facet-options" role="group" :aria-labelledby="`filter-${row.key}-label`">
          <button type="button" class="facet-chip" :class="{ active: draft[row.key] === '' }" :aria-pressed="draft[row.key] === ''" @click="choose(row.key, '')">全部</button>
          <button
            v-for="option in row.options" :key="option.value" type="button" class="facet-chip"
            :class="{ active: draft[row.key] === option.value }" :aria-pressed="draft[row.key] === option.value"
            :title="option.label" @click="choose(row.key, option.value)"
          >{{ option.label }} <span>({{ countLabel(option.count) }})</span></button>
          <template v-if="row.placeholderCount">
            <span v-for="index in row.placeholderCount" :key="`placeholder-${index}`" class="facet-skeleton" aria-hidden="true"></span>
            <span class="sr-only" role="status">正在加载{{ row.label }}选项</span>
          </template>
          <span v-if="row.emptyMessage" class="facet-empty" role="status">{{ row.emptyMessage }}</span>
          <button v-if="row.optionCount > rowLimit" type="button" class="facet-expand" :aria-expanded="!!expanded[row.key]" @click="expanded[row.key] = !expanded[row.key]">
            {{ expanded[row.key] ? '收起' : `展开全部 ${row.optionCount} 项` }}<ChevronUp v-if="expanded[row.key]" :size="12" /><ChevronDown v-else :size="12" />
          </button>
        </div>
      </div>
    </div>
    <div class="filter-bottom">
      <label class="search-field">
        <Search :size="14" />
        <input v-model="draft.keyword" placeholder="搜索站点、POI 或城市" aria-label="搜索站点、POI 或城市"
          @input="scheduleSearch" @compositionstart="beginSearchComposition" @compositionend="endSearchComposition" @keyup.enter="searchOnEnter" />
      </label>
      <label class="filter-sort">排序
        <select v-model="draft.sort" aria-label="站点排序" @change="apply">
          <option value="idleDesc">空闲率从高到低</option><option value="idleAsc">空闲率从低到高</option>
          <option value="priceAsc">电价从低到高</option><option value="priceDesc">电价从高到低</option>
        </select>
      </label>
      <button type="button" class="filter-reset" title="重置筛选" @click="reset"><RotateCcw :size="12" />重置筛选</button>
      <span class="filter-count-note">括号为保留其他条件时的站点数 · 筛选后自动更新</span>
      <button type="button" class="filter-refresh" :title="refreshTitle" :disabled="loading" @click="emit('refresh')"><RefreshCw :size="13" />刷新数据</button>
      <span class="filter-result" role="status">{{ loading ? '正在筛选…' : `共 ${total.toLocaleString()} 个站点` }}</span>
    </div>
  </section>
</template>
