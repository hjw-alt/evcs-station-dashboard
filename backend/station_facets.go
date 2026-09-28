package main

import "strings"

const unknownFacetValue = "__unknown__"

func namedFacet(value string) string {
	if strings.TrimSpace(value) == "" {
		return unknownFacetValue
	}
	return value
}

// Facet counts apply all selected dimensions except the row being counted.
// This allows switching city/operator without losing the other available options.
type StationFacetGroup struct {
	Total  int            `json:"total"`
	Counts map[string]int `json:"counts"`
}

type StationFacets map[string]*StationFacetGroup

func stationDimensions(s Station) map[string]string {
	tou := "none"
	if s.HasTOU {
		tou = "has"
	} else if s.FlatOnly {
		tou = "flat"
	}
	piles := "without"
	if s.HasPileDetails {
		piles = "with"
	}
	return map[string]string{
		"city": namedFacet(s.City), "operator": namedFacet(s.Operator), "availability": stationAvailability(s),
		"priceBand": s.PriceLevel, "tou": tou, "piles": piles,
	}
}

func selectedDimensions(f stationFilters) map[string]string {
	return map[string]string{
		"city": f.City, "operator": f.Operator, "availability": validAvailability(f.Availability),
		"priceBand": f.PriceBand, "tou": f.TOU, "piles": f.Piles,
	}
}

// items have only the keyword SQL predicate applied; pagination happens later.
func filterAndFacetStations(items []Station, filters stationFilters) ([]Station, StationFacets) {
	selected := selectedDimensions(filters)
	facets := make(StationFacets, len(selected))
	for key := range selected {
		facets[key] = &StationFacetGroup{Counts: map[string]int{}}
	}
	matching := make([]Station, 0, len(items))
	for _, station := range items {
		dimensions := stationDimensions(station)
		mismatches, mismatchKey := 0, ""
		for key, value := range selected {
			if value != "" && dimensions[key] != value {
				mismatches++
				mismatchKey = key
			}
		}
		if mismatches == 0 {
			matching = append(matching, station)
		}
		for key, value := range dimensions {
			group := facets[key]
			// Retain zero-count alternatives so users can see and clear restrictive filters.
			if _, ok := group.Counts[value]; !ok {
				group.Counts[value] = 0
			}
			if mismatches == 0 || (mismatches == 1 && mismatchKey == key) {
				group.Total++
				group.Counts[value]++
			}
		}
	}
	return matching, facets
}
