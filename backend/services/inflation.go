package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"dynamics-dashboard/database"
)

// CPIFetcher fetches year -> annual inflation percent for a country.
type CPIFetcher interface {
	FetchCPI(ctx context.Context, countryISO2 string) (map[int]float64, error)
}

// InflationStore is the subset of database.Store inflation needs.
type InflationStore interface {
	SeriesStore
	UpsertInflationRate(ctx context.Context, country string, year int, ratePercent float64, source string) error
	GetInflationRates(ctx context.Context, country string) (map[int]float64, error)
}

// InflationTarget is the response payload for the target line.
type InflationTarget struct {
	Points      []SeriesPoint `json:"points"`
	CountryCode string        `json:"country_code"`
	BaseAmount  float64       `json:"base_amount"`
	BaseCcy     string        `json:"base_currency"`
	BaseDate    string        `json:"base_date"`
}

// EnsureInflation makes sure CPI data for the country is cached. force=true
// refetches from the World Bank (used by the daily cron so newly published
// years appear); otherwise it fetches only when the cache is empty.
func EnsureInflation(ctx context.Context, store InflationStore, wb CPIFetcher, country string, force bool) error {
	if !force {
		cached, err := store.GetInflationRates(ctx, country)
		if err != nil {
			return err
		}
		if len(cached) > 0 {
			return nil
		}
	}
	rates, err := wb.FetchCPI(ctx, country)
	if err != nil {
		return err
	}
	for year, rate := range rates {
		if err := store.UpsertInflationRate(ctx, country, year, rate, "worldbank"); err != nil {
			return err
		}
	}
	return nil
}

// BuildInflationTarget produces the daily "keep up with inflation" line:
// the first salary entry's amount, grown by cumulative annual CPI of the
// salary-currency country (stepping each Jan 1 with the just-ended year's
// published rate), converted to the display currency at each day's rate.
//
// Years not yet published simply don't step — the line stays flat and the
// step appears automatically once the daily refresh ingests the new figure.
func BuildInflationTarget(ctx context.Context, store InflationStore, wb CPIFetcher,
	frank USDSeriesFetcher, nbk DayFetcher,
	entries []database.SalaryEntry, display string, from, to time.Time) (InflationTarget, error) {

	res := InflationTarget{Points: []SeriesPoint{}}
	if len(entries) == 0 {
		return res, nil
	}

	base := entries[0]
	country, ok := CountryForCurrency(base.CurrencyCode)
	if !ok {
		return res, fmt.Errorf("no inflation source for currency %s", base.CurrencyCode)
	}
	res.CountryCode = country
	res.BaseAmount = base.Amount
	res.BaseCcy = base.CurrencyCode
	res.BaseDate = base.EffectiveDate.Format("2006-01-02")

	if err := EnsureInflation(ctx, store, wb, country, false); err != nil {
		return res, fmt.Errorf("ensure inflation for %s: %w", country, err)
	}
	rates, err := store.GetInflationRates(ctx, country)
	if err != nil {
		return res, err
	}

	// Clamp the window: nothing before the baseline entry, nothing after today.
	if from.Before(base.EffectiveDate) {
		from = base.EffectiveDate
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	if to.After(today) {
		to = today
	}
	if from.After(to) {
		return res, nil
	}

	// Rate cursors for converting the target into the display currency.
	needsFX := base.CurrencyCode != display
	var baseCur, displayCur *rateCursor
	if needsFX {
		for _, ccy := range []string{base.CurrencyCode, display} {
			if ccy == "USD" {
				continue
			}
			if err := ensureCoverage(ctx, store, frank, nbk, ccy, from, to); err != nil {
				log.Printf("inflation: coverage for %s: %v", ccy, err)
			}
		}
		load := func(ccy string) (*rateCursor, error) {
			cur := &rateCursor{}
			if ccy == "USD" {
				return cur, nil // handled as constant 1
			}
			pts, err := store.GetRatesForCurrency(ctx, ccy, from, to)
			if err != nil {
				return nil, err
			}
			cur.points = pts
			if rate, rateDate, err := store.GetLatestRateOnOrBefore(ctx, "USD", ccy, from.AddDate(0, 0, -1)); err == nil {
				cur.points = append([]database.RatePoint{{Date: rateDate, Rate: rate}}, cur.points...)
			}
			return cur, nil
		}
		if baseCur, err = load(base.CurrencyCode); err != nil {
			return res, err
		}
		if displayCur, err = load(display); err != nil {
			return res, err
		}
	}
	rateOf := func(cur *rateCursor, ccy string, d time.Time) (float64, bool) {
		if ccy == "USD" {
			return 1, true
		}
		return cur.at(d)
	}

	// Cumulative factor for steps strictly before `from` (steps landing ON a
	// day inside [from, to] are applied by the daily loop below). Each step at
	// Jan 1 of year X applies year X-1's full annual rate — the baseline's
	// starting month is intentionally ignored to keep the model simple.
	factor := 1.0
	for x := base.EffectiveDate.Year() + 1; x <= from.Year(); x++ {
		jan1 := time.Date(x, time.January, 1, 0, 0, 0, 0, time.UTC)
		if !jan1.Before(from) {
			break
		}
		if rate, ok := rates[x-1]; ok {
			factor *= 1 + rate/100
		}
	}

	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		// Step on Jan 1: apply the just-ended year's published inflation.
		if d.Month() == time.January && d.Day() == 1 {
			if rate, ok := rates[d.Year()-1]; ok && d.Year()-1 >= base.EffectiveDate.Year() {
				factor *= 1 + rate/100
			}
		}
		target := base.Amount * factor
		if needsFX {
			bRate, ok1 := rateOf(baseCur, base.CurrencyCode, d)
			dRate, ok2 := rateOf(displayCur, display, d)
			if !ok1 || !ok2 {
				continue
			}
			target = target / bRate * dRate
		}
		res.Points = append(res.Points, SeriesPoint{Date: d.Format("2006-01-02"), Value: target})
	}
	return res, nil
}
