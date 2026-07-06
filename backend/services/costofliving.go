package services

import (
	_ "embed"
	"encoding/json"
	"strings"
)

// Cost-of-living dataset: static, committed, embedded in the binary
// (mirrors the FX RateSource rule — no external calls, ever). Indices are
// approximate, NYC = 100; see source_note in the JSON.

//go:embed data/cost_of_living.json
var colRaw []byte

type colCity struct {
	City    string  `json:"city"`
	Country string  `json:"country"`
	Index   float64 `json:"index"`
}

type colCountry struct {
	Country string  `json:"country"`
	Index   float64 `json:"index"`
}

type colDataset struct {
	Updated    string       `json:"updated"`
	SourceNote string       `json:"source_note"`
	Cities     []colCity    `json:"cities"`
	Countries  []colCountry `json:"countries"`
}

var colData = mustLoadCOL()

func mustLoadCOL() colDataset {
	var d colDataset
	if err := json.Unmarshal(colRaw, &d); err != nil {
		// Embedded + committed: malformed data must fail loudly at startup
		// (and in every test run), never at request time.
		panic("cost_of_living.json is malformed: " + err.Error())
	}
	return d
}

func colKey(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// COLUpdated returns the dataset's manual-update date (surfaced in the UI
// disclaimer so staleness is visible).
func COLUpdated() string { return colData.Updated }

// COLSourceNote returns the dataset's approximation disclaimer.
func COLSourceNote() string { return colData.SourceNote }

// COLCities lists known cities (for the frontend datalist).
func COLCities() []colCity { return colData.Cities }

// LookupCOL resolves a location to a cost-of-living index.
// Resolution order (feature plan §3): exact city match (case/whitespace
// insensitive) → country fallback → none. The caller derives countryCode
// from the salary currency (CountryForCurrency).
func LookupCOL(city, countryCode string) (index float64, matched bool, level string) {
	if key := colKey(city); key != "" {
		for _, c := range colData.Cities {
			if colKey(c.City) == key {
				return c.Index, true, "city"
			}
		}
	}
	if countryCode != "" {
		for _, c := range colData.Countries {
			if strings.EqualFold(c.Country, countryCode) {
				return c.Index, true, "country"
			}
		}
	}
	return 0, false, "none"
}
