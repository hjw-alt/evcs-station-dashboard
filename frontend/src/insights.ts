import type { Availability, StationSummary } from './types'

export const availabilityLabels: Record<Availability, string> = {
  idle: '空闲', moderate: '适中', full: '满载', unknown: '无数据',
}

// Use the API snapshot clock and capture timestamp, never browser refresh/ingestion time.
export function captureFreshness(capturedAt: number, summary: StationSummary) {
  if (!Number.isFinite(capturedAt) || capturedAt <= 0 || capturedAt > summary.asOf) return 'unknown'
  return summary.asOf - capturedAt <= summary.freshnessWindowSeconds ? 'recent' : 'stale'
}
