package services

import (
	"context"
	"fmt"
	"log"
	"time"
)

// IngestResult summarizes one refresh run.
type IngestResult struct {
	FrankfurterRows int `json:"frankfurter_rows"`
	NBKRows         int `json:"nbk_rows"`
}

// Ingest bulk-fetches historical rates into the store:
//   - Frankfurter: full [start, end] daily series, all currencies (one request)
//   - NBK: last 14 days ending at end (one request per day)
//
// Older KZT/NBK dates are filled on demand by the Converter when needed.
func Ingest(ctx context.Context, store RateStore, frank USDSeriesFetcher, nbk DayFetcher, start, end time.Time) (IngestResult, error) {
	var res IngestResult

	series, err := frank.FetchUSDSeries(ctx, start, end, nil)
	if err != nil {
		return res, fmt.Errorf("frankfurter ingest: %w", err)
	}
	for dateStr, symbols := range series {
		d, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		for symbol, rate := range symbols {
			if err := store.UpsertRate(ctx, d, "USD", symbol, rate, "frankfurter"); err != nil {
				return res, err
			}
			res.FrankfurterRows++
		}
	}

	nbkStart := end.AddDate(0, 0, -14)
	if nbkStart.Before(start) {
		nbkStart = start
	}
	for d := nbkStart; !d.After(end); d = d.AddDate(0, 0, 1) {
		day, err := nbk.FetchDay(ctx, d)
		if err != nil {
			log.Printf("nbk ingest %s: %v (skipping day)", d.Format("2006-01-02"), err)
			continue
		}
		for code, rate := range day {
			if err := store.UpsertRate(ctx, d, "USD", code, rate, "nbk"); err != nil {
				return res, err
			}
			res.NBKRows++
		}
	}
	return res, nil
}
