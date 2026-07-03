package services

import (
	"context"
	"log"
	"sync"
	"time"

	"dynamics-dashboard/database"
)

// SeriesPoint is one day of the salary-dynamics line: the salary in effect
// that day, converted at that day's exchange rate (carry-forward on gaps).
type SeriesPoint struct {
	Date  string  `json:"date"`
	Value float64 `json:"value"`
}

// SeriesStore is the subset of database.Store the series builder needs.
type SeriesStore interface {
	RateStore
	GetRatesForCurrency(ctx context.Context, quote string, from, to time.Time) ([]database.RatePoint, error)
	GetRateCoverage(ctx context.Context, quote string, from, to time.Time) (time.Time, time.Time, int64, error)
}

// rateCursor walks a sorted rate slice day by day, carrying the last known
// rate forward across weekends/holidays and sampled gaps.
type rateCursor struct {
	points []database.RatePoint
	idx    int
	cur    float64
	has    bool
}

func (c *rateCursor) at(d time.Time) (float64, bool) {
	for c.idx < len(c.points) && !c.points[c.idx].Date.After(d) {
		c.cur = c.points[c.idx].Rate
		c.has = true
		c.idx++
	}
	return c.cur, c.has
}

// BuildDailySeries produces one point per day in [from, to]: the salary entry
// in effect that day converted to display using that day's USD-cross rates.
func BuildDailySeries(ctx context.Context, store SeriesStore, frank USDSeriesFetcher, nbk DayFetcher,
	entries []database.SalaryEntry, display string, from, to time.Time) ([]SeriesPoint, error) {

	points := []SeriesPoint{}
	if len(entries) == 0 {
		return points, nil
	}

	// Clamp: nothing before the first salary entry, nothing after today.
	first := entries[0].EffectiveDate
	if from.Before(first) {
		from = first
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	if to.After(today) {
		to = today
	}
	if from.After(to) {
		return points, nil
	}

	// Collect the currencies that actually need rates: pairs only matter for
	// entries whose currency differs from the display currency (USD is the
	// cross base and always rate 1).
	currencies := map[string]bool{}
	for _, e := range entries {
		if e.CurrencyCode == display {
			continue
		}
		if e.CurrencyCode != "USD" {
			currencies[e.CurrencyCode] = true
		}
		if display != "USD" {
			currencies[display] = true
		}
	}

	// Make sure the DB has rates across the range (bulk backfill on first use).
	for ccy := range currencies {
		if err := ensureCoverage(ctx, store, frank, nbk, ccy, from, to); err != nil {
			log.Printf("series: coverage for %s: %v", ccy, err)
		}
	}

	// Load rates once, seeding each cursor with the last rate before `from`.
	cursors := map[string]*rateCursor{}
	for ccy := range currencies {
		pts, err := store.GetRatesForCurrency(ctx, ccy, from, to)
		if err != nil {
			return nil, err
		}
		cur := &rateCursor{points: pts}
		if rate, rateDate, err := store.GetLatestRateOnOrBefore(ctx, "USD", ccy, from.AddDate(0, 0, -1)); err == nil {
			cur.points = append([]database.RatePoint{{Date: rateDate, Rate: rate}}, pts...)
		}
		cursors[ccy] = cur
	}
	rateAt := func(ccy string, d time.Time) (float64, bool) {
		if ccy == "USD" {
			return 1, true
		}
		return cursors[ccy].at(d)
	}

	entryIdx := 0
	var active *database.SalaryEntry
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		for entryIdx < len(entries) && !entries[entryIdx].EffectiveDate.After(d) {
			active = &entries[entryIdx]
			entryIdx++
		}
		if active == nil {
			continue
		}
		var value float64
		if active.CurrencyCode == display {
			value = active.Amount // no conversion needed
		} else {
			fromRate, ok1 := rateAt(active.CurrencyCode, d)
			toRate, ok2 := rateAt(display, d)
			if !ok1 || !ok2 {
				continue // no rate known yet this early — line starts when rates exist
			}
			value = active.Amount / fromRate * toRate
		}
		points = append(points, SeriesPoint{
			Date:  d.Format("2006-01-02"),
			Value: value,
		})
	}
	return points, nil
}

// ensureCoverage backfills USD->ccy rates for [from, to] when the DB is missing
// the head or tail of the range. Frankfurter needs one bulk call; NBK is
// fetched per day with adaptive sampling (older data -> sparser samples,
// carry-forward fills the gaps).
func ensureCoverage(ctx context.Context, store SeriesStore, frank USDSeriesFetcher, nbk DayFetcher,
	ccy string, from, to time.Time) error {

	minD, maxD, count, err := store.GetRateCoverage(ctx, ccy, from, to)
	if err != nil {
		return err
	}
	// Coverage must be dense, not just touch both ends: sparse on-demand
	// windows (a few days around each entry) must not suppress the backfill,
	// or the "daily" line goes flat for months. Sampling is at worst biweekly,
	// so require at least ~1 rate per 20 days across the span.
	spanDays := to.Sub(from).Hours() / 24
	minCount := int64(spanDays/20) + 2
	needFrom, needTo := from, to
	if count >= minCount {
		headOK := !minD.After(from.AddDate(0, 0, 10))
		tailOK := !maxD.Before(to.AddDate(0, 0, -10))
		if headOK && tailOK {
			return nil
		}
		if headOK {
			needFrom = maxD // dense, only the tail is missing
		} else if tailOK {
			needTo = minD // dense, only the head is missing
		}
	}

	if FrankfurterCurrencies[ccy] && frank != nil {
		series, err := frank.FetchUSDSeries(ctx, needFrom.AddDate(0, 0, -7), needTo, []string{ccy})
		if err != nil {
			return err
		}
		for dateStr, symbols := range series {
			d, err := time.Parse("2006-01-02", dateStr)
			if err != nil {
				continue
			}
			for symbol, rate := range symbols {
				if err := store.UpsertRate(ctx, d, "USD", symbol, rate, "frankfurter"); err != nil {
					return err
				}
			}
		}
		return nil
	}

	if nbk != nil {
		dates := sampledDates(needFrom.AddDate(0, 0, -7), needTo)
		log.Printf("series: backfilling %s from NBK (%d sampled days)", ccy, len(dates))
		fetchNBKParallel(ctx, store, nbk, dates)
	}
	return nil
}

// sampledDates picks fetch dates with age-based density: daily for the last
// 90 days, weekly up to a year back, biweekly beyond that.
func sampledDates(from, to time.Time) []time.Time {
	now := time.Now().UTC()
	var dates []time.Time
	for d := from; !d.After(to); {
		dates = append(dates, d)
		age := now.Sub(d)
		switch {
		case age > 365*24*time.Hour:
			d = d.AddDate(0, 0, 14)
		case age > 90*24*time.Hour:
			d = d.AddDate(0, 0, 7)
		default:
			d = d.AddDate(0, 0, 1)
		}
	}
	return dates
}

// fetchNBKParallel fetches the sampled days with a small worker pool and
// upserts every currency NBK returns (KZT, RUB, and ~40 others per day).
func fetchNBKParallel(ctx context.Context, store SeriesStore, nbk DayFetcher, dates []time.Time) {
	sem := make(chan struct{}, 6)
	var wg sync.WaitGroup
	for _, d := range dates {
		wg.Add(1)
		sem <- struct{}{}
		go func(day time.Time) {
			defer wg.Done()
			defer func() { <-sem }()
			rates, err := nbk.FetchDay(ctx, day)
			if err != nil {
				log.Printf("series: nbk %s: %v", day.Format("2006-01-02"), err)
				return
			}
			for code, rate := range rates {
				if err := store.UpsertRate(ctx, day, "USD", code, rate, "nbk"); err != nil {
					log.Printf("series: upsert %s %s: %v", code, day.Format("2006-01-02"), err)
					return
				}
			}
		}(d)
	}
	wg.Wait()
}
