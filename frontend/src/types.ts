export interface Overview {
  total: number
  priced: number
  withTou: number
  flatOnly: number
  noTou: number
  noPrice: number
  withPiles: number
  lowPriceAvailable: number
  averagePrice: number
  pileTotal: number
  pileIdle: number
  pileBusy: number
  pileUnknown: number
  updatedAt: number
  touCoverage: number
  pileCoverage: number
  idleRate: number
  busyRate: number
}

export interface Station {
  sourceKey: string
  sourceStationId: string
  matchedName: string
  requestedName: string
  city: string
  district: string
  operator: string
  address: string
  currentPriceText: string
  currentPrice: number
  pricePercentile: number
  priceLevel: 'cheap' | 'mid' | 'expensive' | 'missing'
  pricePeriodCount: number
  hasTou: boolean
  flatOnly: boolean
  noPriceData: boolean
  fastIdle: number
  fastBusy: number
  fastTotal: number
  superIdle: number
  superBusy: number
  superTotal: number
  slowIdle: number
  slowBusy: number
  slowTotal: number
  pileIdle: number
  pileBusy: number
  pileUnknown: number
  pileTotal: number
  hasPileDetails: boolean
  idleRate: number
  capturedAt: number
  receivedAt: number
  collectionSource: string
  fastPower: string
  superPower: string
  slowPower: string
}

export interface MapStation {
  sourceKey: string
  name: string
  city: string
  district: string
  address: string
  operator: string
  longitude: number
  latitude: number
  currentPrice: number
  currentPriceText: string
  priceLevel: 'cheap' | 'mid' | 'expensive' | 'missing'
  pricePercentile: number
  pileIdle: number
  pileBusy: number
  pileUnknown: number
  pileTotal: number
  idleRate: number
  receivedAt: number
}

export interface MapStationResponse {
  items: MapStation[]
  total: number
  updatedAt: number
}

export interface PricePeriod {
  time: string
  elecFee: string
  serviceFee: string
  totalPrice: string
  tag: string
}

export interface PileDetail {
  deviceId: string
  chargingType: string
  equipmentType: string
  ratedPower: string
  ratedCurrent: string
  ratedVoltage: string
  standard: string
  status: string
}

export interface StationDetail extends Station {
  businessHours: string
  parkingFee: string
  fastPrices: PricePeriod[]
  slowPrices: PricePeriod[]
  piles: PileDetail[]
  payload: Record<string, unknown>
  servedAt: number
}

export interface HistoryPoint {
  capturedAt: number
  receivedAt: number
  currentPrice: number
  pileIdle: number
  pileBusy: number
  pileUnknown: number
  pileTotal: number
  isChanged: boolean
}

export interface Facet {
  name: string
  count: number
}

export interface Meta {
  cities: Facet[]
  operators: Facet[]
}

export type Availability = 'idle' | 'moderate' | 'full' | 'unknown'

export interface StationSummary {
  total: number
  availability: Record<Availability, number>
  topAvailable: Station[]
  freshness: {
    recent: number
    stale: number
    unknown: number
    missingPrice: number
    missingPiles: number
  }
  asOf: number
  freshnessWindowSeconds: number
}

export interface StationResponse {
  summary: StationSummary
  items: Station[]
  total: number
  page: number
  pageSize: number
}

export interface StationFilters {
  availability: Availability | ''
  keyword: string
  city: string
  operator: string
  tou: string
  priceBand: string
  piles: string
  sort: string
  page: number
  pageSize: number
}
