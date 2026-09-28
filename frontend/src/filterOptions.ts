import type { StationFacets } from './types'

export type FilterOptionNames = Record<'city' | 'operator', string[]>
export const FILTER_OPTION_CACHE_KEY = 'station-dashboard:filter-options:v1'

// Only cache option names, never counts: counts belong to the current filter snapshot.
export function readFilterOptions(): FilterOptionNames {
  const empty = { city: [], operator: [] }
  try {
    const cached = JSON.parse(sessionStorage.getItem(FILTER_OPTION_CACHE_KEY) ?? 'null')
    if (!cached || typeof cached !== 'object') return empty
    const names = (value: unknown): string[] => Array.isArray(value)
      ? [...new Set(value.filter((name): name is string => typeof name === 'string' && !!name.trim() && name.length <= 200))].slice(0, 500)
      : []
    return { city: names(cached.city), operator: names(cached.operator) }
  } catch {
    // Storage may be unavailable; first-load skeletons still reserve the option space.
    return empty
  }
}

export function mergeFilterOptions(previous: FilterOptionNames, facets: StationFacets): FilterOptionNames {
  const merge = (key: keyof FilterOptionNames) => {
    const counts = facets[key]?.counts ?? {}
    const names = Object.keys(counts).filter(Boolean).sort((a, b) => counts[b]! - counts[a]! || a.localeCompare(b, 'zh-CN'))
    // Preserve known labels/order across refreshes and zero-result keyword searches.
    return [...new Set([...previous[key], ...names])].slice(0, 500)
  }
  return { city: merge('city'), operator: merge('operator') }
}

export function saveFilterOptions(names: FilterOptionNames) {
  try {
    sessionStorage.setItem(FILTER_OPTION_CACHE_KEY, JSON.stringify(names))
  } catch {
    // Keeping the in-memory labels is enough for in-page refreshes.
  }
}
