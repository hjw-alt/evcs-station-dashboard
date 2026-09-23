package main

import "sort"

const freshnessWindowSeconds int64 = 24 * 60 * 60

type AvailabilityCounts struct {
	Idle     int `json:"idle"`
	Moderate int `json:"moderate"`
	Full     int `json:"full"`
	Unknown  int `json:"unknown"`
}

type FreshnessCounts struct {
	Recent       int `json:"recent"`
	Stale        int `json:"stale"`
	Unknown      int `json:"unknown"`
	MissingPrice int `json:"missingPrice"`
	MissingPiles int `json:"missingPiles"`
}

type StationSummary struct {
	Total                  int                `json:"total"`
	Availability           AvailabilityCounts `json:"availability"`
	TopAvailable           []Station          `json:"topAvailable"`
	Freshness              FreshnessCounts    `json:"freshness"`
	AsOf                   int64              `json:"asOf"`
	FreshnessWindowSeconds int64              `json:"freshnessWindowSeconds"`
}

func validAvailability(value string) string {
	switch value {
	case "idle", "moderate", "full", "unknown":
		return value
	default:
		return ""
	}
}

// Keep the thresholds identical to the card colors, including no-detail sites.
func stationAvailability(station Station) string {
	if !station.HasPileDetails || station.PileTotal <= 0 {
		return "unknown"
	}
	if station.IdleRate >= 50 {
		return "idle"
	}
	if station.IdleRate >= 20 {
		return "moderate"
	}
	return "full"
}

func summarizeStations(items []Station, now int64) StationSummary {
	summary := StationSummary{
		Total: len(items), AsOf: now, FreshnessWindowSeconds: freshnessWindowSeconds,
		TopAvailable: make([]Station, 0, 5),
	}
	eligible := make([]Station, 0)
	for _, station := range items {
		switch stationAvailability(station) {
		case "idle":
			summary.Availability.Idle++
		case "moderate":
			summary.Availability.Moderate++
		case "full":
			summary.Availability.Full++
		default:
			summary.Availability.Unknown++
		}
		// receivedAt is ingestion time: it must not make an old capture look fresh.
		switch {
		case station.CapturedAt <= 0 || station.CapturedAt > now:
			summary.Freshness.Unknown++
		case now-station.CapturedAt <= freshnessWindowSeconds:
			summary.Freshness.Recent++
		default:
			summary.Freshness.Stale++
		}
		if station.CurrentPrice <= 0 {
			summary.Freshness.MissingPrice++
		}
		if stationAvailability(station) == "unknown" {
			summary.Freshness.MissingPiles++
		}
		if station.CurrentPrice > 0 && station.HasPileDetails && station.PileTotal > 0 && station.PileIdle > 0 {
			eligible = append(eligible, station)
		}
	}
	sort.Slice(eligible, func(i, j int) bool {
		a, b := eligible[i], eligible[j]
		if a.CurrentPrice != b.CurrentPrice {
			return a.CurrentPrice < b.CurrentPrice
		}
		if a.PileIdle != b.PileIdle {
			return a.PileIdle > b.PileIdle
		}
		if a.IdleRate != b.IdleRate {
			return a.IdleRate > b.IdleRate
		}
		return a.SourceKey < b.SourceKey
	})
	summary.TopAvailable = append(summary.TopAvailable, eligible[:min(5, len(eligible))]...)
	return summary
}

func stationResponse(items []Station, filters stationFilters, now int64) StationResponse {
	matching := make([]Station, 0, len(items))
	availability := validAvailability(filters.Availability)
	for _, station := range items {
		if availability == "" || stationAvailability(station) == availability {
			matching = append(matching, station)
		}
	}
	summary := summarizeStations(matching, now)
	if filters.Sort == "idleDesc" {
		matching = idleOrderedPage(matching, filters.Page, filters.PageSize)
	} else {
		// Other sort orders are already applied by the SQL query.
		matching = stationPage(matching, filters.Page, filters.PageSize)
	}
	return StationResponse{Items: matching, Total: summary.Total, Page: filters.Page, PageSize: filters.PageSize, Summary: summary}
}
