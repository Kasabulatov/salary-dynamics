package services

import (
	"context"
	"log"
	"math"
	"sort"
	"time"

	"dynamics-dashboard/database"
)

// Significant-change thresholds (spec §2.3: > 2% daily or weekly).
const (
	eventThreshold = 0.02
	maxDailyGap    = 3 * 24 * time.Hour  // consecutive samples further apart aren't a "daily" move
	weeklyLookback = 7 * 24 * time.Hour  // compare against the rate ~a week earlier
	weeklyWindow   = 16 * 24 * time.Hour // ...found at most 16 days back (sampled history)
)

// EventsStore is the subset of database.Store event detection needs.
type EventsStore interface {
	SeriesStore
	ReplaceEvents(ctx context.Context, base, quote string, from, to time.Time, events []database.CurrencyEvent) error
	GetEvents(ctx context.Context, base, quote string, from, to time.Time) ([]database.CurrencyEvent, error)
}

// RecomputeEvents ensures rate coverage for the pair, rebuilds the cross-rate
// point series (quote units per 1 base), detects >2% daily/weekly moves, and
// replaces the cached events for the range. Returns the fresh events.
func RecomputeEvents(ctx context.Context, store EventsStore, frank USDSeriesFetcher, nbk DayFetcher,
	base, quote string, from, to time.Time) ([]database.CurrencyEvent, error) {

	if base == quote {
		return []database.CurrencyEvent{}, nil
	}

	// Rates must exist first (same backfill the series endpoint uses).
	// Extend the head so the first in-range day has daily/weekly context.
	rateFrom := from.AddDate(0, 0, -21)
	for _, ccy := range []string{base, quote} {
		if ccy == "USD" {
			continue
		}
		if err := ensureCoverage(ctx, store, frank, nbk, ccy, rateFrom, to); err != nil {
			log.Printf("events: coverage for %s: %v", ccy, err)
		}
	}

	cross, err := crossRatePoints(ctx, store, base, quote, rateFrom, to)
	if err != nil {
		return nil, err
	}

	events := detectEvents(cross, from, to)
	if err := store.ReplaceEvents(ctx, base, quote, from, to, events); err != nil {
		return nil, err
	}
	return store.GetEvents(ctx, base, quote, from, to)
}

// crossRatePoints builds the raw (not carried-forward) series of the cross
// rate: units of quote per 1 base. Points exist only on dates where at least
// one underlying USD rate was actually sampled — so gap artifacts can't
// masquerade as daily moves.
func crossRatePoints(ctx context.Context, store SeriesStore, base, quote string, from, to time.Time) ([]database.RatePoint, error) {
	load := func(ccy string) ([]database.RatePoint, error) {
		if ccy == "USD" {
			return nil, nil // constant 1, no own dates
		}
		return store.GetRatesForCurrency(ctx, ccy, from, to)
	}
	basePts, err := load(base)
	if err != nil {
		return nil, err
	}
	quotePts, err := load(quote)
	if err != nil {
		return nil, err
	}

	// Union of sample dates from both series.
	dateSet := map[string]time.Time{}
	for _, p := range basePts {
		dateSet[p.Date.Format("2006-01-02")] = p.Date
	}
	for _, p := range quotePts {
		dateSet[p.Date.Format("2006-01-02")] = p.Date
	}
	dates := make([]time.Time, 0, len(dateSet))
	for _, d := range dateSet {
		dates = append(dates, d)
	}
	sort.Slice(dates, func(i, j int) bool { return dates[i].Before(dates[j]) })

	baseCur := &rateCursor{points: basePts}
	quoteCur := &rateCursor{points: quotePts}
	rateOf := func(cur *rateCursor, ccy string, d time.Time) (float64, bool) {
		if ccy == "USD" {
			return 1, true
		}
		return cur.at(d)
	}

	var cross []database.RatePoint
	for _, d := range dates {
		b, ok1 := rateOf(baseCur, base, d)
		q, ok2 := rateOf(quoteCur, quote, d)
		if !ok1 || !ok2 {
			continue
		}
		// (quote per USD) / (base per USD) = quote units per 1 base.
		cross = append(cross, database.RatePoint{Date: d, Rate: q / b})
	}
	return cross, nil
}

// detectEvents scans cross-rate points for >2% daily and weekly moves.
func detectEvents(points []database.RatePoint, from, to time.Time) []database.CurrencyEvent {
	events := []database.CurrencyEvent{}
	inRange := func(d time.Time) bool { return !d.Before(from) && !d.After(to) }

	for i := 1; i < len(points); i++ {
		p, prev := points[i], points[i-1]
		if !inRange(p.Date) || prev.Rate == 0 {
			continue
		}
		if p.Date.Sub(prev.Date) <= maxDailyGap {
			pct := p.Rate/prev.Rate - 1
			if math.Abs(pct) >= eventThreshold {
				events = append(events, database.CurrencyEvent{
					Date:           p.Date,
					ChangeWindow:   "daily",
					PercentChange:  pct * 100,
					AbsoluteChange: p.Rate - prev.Rate,
				})
			}
		}
	}

	var weekly []database.CurrencyEvent
	for i := range points {
		p := points[i]
		if !inRange(p.Date) {
			continue
		}
		// Latest point at least a week before p, within the sampled window.
		var ref *database.RatePoint
		for j := i - 1; j >= 0; j-- {
			age := p.Date.Sub(points[j].Date)
			if age < weeklyLookback {
				continue
			}
			if age > weeklyWindow {
				break
			}
			ref = &points[j]
			break
		}
		if ref == nil || ref.Rate == 0 {
			continue
		}
		pct := p.Rate/ref.Rate - 1
		if math.Abs(pct) >= eventThreshold {
			weekly = append(weekly, database.CurrencyEvent{
				Date:           p.Date,
				ChangeWindow:   "weekly",
				PercentChange:  pct * 100,
				AbsoluteChange: p.Rate - ref.Rate,
			})
		}
	}
	return append(events, coalesceRuns(weekly)...)
}

// coalesceRuns collapses streaks of consecutive weekly events (a sustained
// move re-triggers day after day against its rolling week-ago baseline) into
// a single marker: the strongest event of each run.
func coalesceRuns(events []database.CurrencyEvent) []database.CurrencyEvent {
	if len(events) == 0 {
		return events
	}
	out := []database.CurrencyEvent{}
	best := events[0]
	prev := events[0]
	for _, e := range events[1:] {
		sameRun := e.Date.Sub(prev.Date) <= 2*24*time.Hour &&
			(e.PercentChange >= 0) == (best.PercentChange >= 0)
		if sameRun {
			if math.Abs(e.PercentChange) > math.Abs(best.PercentChange) {
				best = e
			}
		} else {
			out = append(out, best)
			best = e
		}
		prev = e
	}
	return append(out, best)
}
