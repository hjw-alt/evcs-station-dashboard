package main

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestStationAvailabilityThresholds(t *testing.T) {
	for _, tc := range []struct {
		name    string
		details bool
		total   int
		rate    float64
		want    string
	}{
		{"no-details", false, 10, 100, "unknown"},
		{"zero-total", true, 0, 100, "unknown"},
		{"idle-boundary", true, 10, 50, "idle"},
		{"below-idle", true, 10, 49.9, "moderate"},
		{"moderate-boundary", true, 10, 20, "moderate"},
		{"below-moderate", true, 10, 19.9, "full"},
		{"busy", true, 10, 0, "full"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := stationAvailability(Station{HasPileDetails: tc.details, PileTotal: tc.total, IdleRate: tc.rate})
			if got != tc.want {
				t.Fatalf("availability = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestSummaryFreshnessUsesCaptureNotIngestion(t *testing.T) {
	const now int64 = 2_000_000
	items := []Station{
		{CapturedAt: now},
		{CapturedAt: now - freshnessWindowSeconds},
		{CapturedAt: now - freshnessWindowSeconds - 1, ReceivedAt: now},
		{CapturedAt: 0, ReceivedAt: now},
		{CapturedAt: -1},
		{CapturedAt: now + 1},
	}
	summary := summarizeStations(items, now)
	want := FreshnessCounts{Recent: 2, Stale: 1, Unknown: 3, MissingPrice: 6, MissingPiles: 6}
	if summary.Freshness != want {
		t.Fatalf("freshness = %+v, want %+v", summary.Freshness, want)
	}
	if summary.AsOf != now || summary.FreshnessWindowSeconds != 86400 {
		t.Fatal("missing capture reference time/window")
	}
	if summary.Availability.Unknown != 6 {
		t.Fatal("missing pile data must stay unknown")
	}
}

func TestSummaryTopFiveEligibilityAndStablePriceOrder(t *testing.T) {
	makeStation := func(key string, price float64, idle int, rate float64) Station {
		return Station{SourceKey: key, CurrentPrice: price, HasPileDetails: true, PileTotal: 100, PileIdle: idle, IdleRate: rate}
	}
	items := []Station{
		makeStation("expensive", 10, 100, 100),
		makeStation("same-price-fewer-piles", 0.8, 5, 100),
		makeStation("b-tie", 0.8, 8, 80),
		makeStation("cheap", 0.5, 1, 1),
		makeStation("a-tie", 0.8, 8, 80),
		makeStation("same-piles-lower-rate", 0.8, 8, 70),
		makeStation("no-price", 0, 8, 80),
		makeStation("negative-price", -1, 8, 80),
		makeStation("no-idle", 0.1, 0, 0),
		{SourceKey: "no-details", CurrentPrice: 0.1, PileTotal: 10, PileIdle: 10},
		{SourceKey: "zero-total", CurrentPrice: 0.1, HasPileDetails: true, PileIdle: 10},
	}
	original := slices.Clone(items)
	summary := summarizeStations(items, 2_000_000)
	want := []string{"cheap", "a-tie", "b-tie", "same-piles-lower-rate", "same-price-fewer-piles"}
	if got := stationKeys(summary.TopAvailable); !reflect.DeepEqual(got, want) {
		t.Fatalf("top five = %v, want %v", got, want)
	}
	if !reflect.DeepEqual(items, original) {
		t.Fatal("summary must not reorder the SQL result")
	}
	slices.Reverse(items)
	if got := stationKeys(summarizeStations(items, 2_000_000).TopAvailable); !reflect.DeepEqual(got, want) {
		t.Fatal("ties are not deterministic")
	}
	if summary.Total != len(items) || summary.Freshness.MissingPrice != 2 || summary.Freshness.MissingPiles != 2 {
		t.Fatalf("quality totals = %+v", summary)
	}
}

func TestSummaryCoversAll2523ResultsBeforeEveryPageAndSort(t *testing.T) {
	items := make([]Station, 2523)
	for i := range items {
		items[i] = Station{SourceKey: fmt.Sprintf("station-%04d", i), CurrentPrice: float64(i+1) / 100,
			HasPileDetails: true, PileTotal: 100, PileIdle: i % 101, IdleRate: float64(i % 101), CapturedAt: 2_000_000}
	}
	want := summarizeStations(items, 2_000_000)
	for _, sortBy := range []string{"idleDesc", "priceAsc", "priceDesc", "periods", "name", "updated"} {
		for _, page := range []int{1, 2, 101, 102} {
			response := stationResponse(items, stationFilters{Sort: sortBy, Page: page, PageSize: 25}, 2_000_000)
			if response.Total != 2523 || !reflect.DeepEqual(response.Summary, want) {
				t.Fatalf("%s page %d summary is page-limited", sortBy, page)
			}
			start := min((page-1)*25, len(items))
			size := min(25, len(items)-start)
			if len(response.Items) != size {
				t.Fatalf("%s page %d size = %d, want %d", sortBy, page, len(response.Items), size)
			}
			if sortBy != "idleDesc" && !reflect.DeepEqual(stationKeys(response.Items), stationKeys(items[start:start+size])) {
				t.Fatal("SQL sort order must survive summary calculation")
			}
		}
	}
}

func TestAvailabilityFilterBeforeSummaryAndPagination(t *testing.T) {
	items := []Station{
		{SourceKey: "idle-a", HasPileDetails: true, PileTotal: 10, PileIdle: 10, IdleRate: 100, CurrentPrice: 1},
		{SourceKey: "moderate", HasPileDetails: true, PileTotal: 10, PileIdle: 3, IdleRate: 30, CurrentPrice: .1},
		{SourceKey: "idle-b", HasPileDetails: true, PileTotal: 10, PileIdle: 5, IdleRate: 50, CurrentPrice: .5},
		{SourceKey: "full", HasPileDetails: true, PileTotal: 10},
		{SourceKey: "unknown"},
	}
	for _, category := range []string{"idle", "moderate", "full", "unknown"} {
		response := stationResponse(items, stationFilters{Availability: category, Sort: "idleDesc", Page: 1, PageSize: 1}, 2_000_000)
		want := 1
		if category == "idle" {
			want = 2
		}
		if response.Total != want || response.Summary.Total != want || len(response.Items) != 1 {
			t.Fatalf("bad %s totals", category)
		}
		for _, station := range append(response.Items, response.Summary.TopAvailable...) {
			if stationAvailability(station) != category {
				t.Fatalf("%s filter included %s", category, station.SourceKey)
			}
		}
	}
	response := stationResponse(items, stationFilters{Availability: "idle", Page: 2, PageSize: 1}, 2_000_000)
	if response.Items[0].SourceKey != "idle-b" || response.Summary.Availability.Idle != 2 {
		t.Fatal("filter must precede pagination")
	}
}

func TestEmptySummarySerializesArraysAndZeroCounts(t *testing.T) {
	response := stationResponse(nil, stationFilters{Page: 1, PageSize: 25}, 2_000_000)
	if response.Items == nil || response.Summary.TopAvailable == nil || response.Total != 0 {
		t.Fatal("empty result must use empty arrays")
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"items":[]`, `"topAvailable":[]`, `"total":0`} {
		if !strings.Contains(string(encoded), field) {
			t.Fatalf("missing %s in %s", field, encoded)
		}
	}
}

func TestParseAvailabilityWhitelist(t *testing.T) {
	for _, value := range []string{"idle", "moderate", "full", "unknown", "invalid", ""} {
		filters := parseStationFilters(httptest.NewRequest("GET", "/api/stations?availability="+value, nil))
		want := value
		if value == "invalid" {
			want = ""
		}
		if filters.Availability != want {
			t.Fatalf("filter %s became %s", value, filters.Availability)
		}
	}
}
