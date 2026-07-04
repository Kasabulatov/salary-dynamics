package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// WorldBank fetches annual CPI inflation (indicator FP.CPI.TOTL.ZG) — free,
// no API key, covers every country with one adapter.
type WorldBank struct {
	BaseURL string
	Client  *http.Client
}

func NewWorldBank() *WorldBank {
	return &WorldBank{
		BaseURL: "https://api.worldbank.org",
		Client:  &http.Client{Timeout: 15 * time.Second},
	}
}

type wbEntry struct {
	Date  string   `json:"date"` // year, e.g. "2024"
	Value *float64 `json:"value"`
}

// FetchCPI returns year -> annual inflation percent for a country.
// Unpublished years (null values) are omitted.
func (w *WorldBank) FetchCPI(ctx context.Context, countryISO2 string) (map[int]float64, error) {
	url := fmt.Sprintf("%s/v2/country/%s/indicator/FP.CPI.TOTL.ZG?format=json&per_page=100",
		w.BaseURL, countryISO2)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := w.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("worldbank request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("worldbank returned status %d", resp.StatusCode)
	}

	// Response shape: [ {page metadata}, [ {date, value, ...}, ... ] ]
	var raw []json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("worldbank decode: %w", err)
	}
	if len(raw) < 2 {
		return nil, fmt.Errorf("worldbank: unexpected response shape")
	}
	var entries []wbEntry
	if err := json.Unmarshal(raw[1], &entries); err != nil {
		return nil, fmt.Errorf("worldbank decode entries: %w", err)
	}

	out := map[int]float64{}
	for _, e := range entries {
		if e.Value == nil {
			continue
		}
		year, err := strconv.Atoi(e.Date)
		if err != nil {
			continue
		}
		out[year] = *e.Value
	}
	return out, nil
}
