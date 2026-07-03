package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"dynamics-dashboard/database"
)

// fetchWindow is how far back we fetch from a source when the cache has no
// rate at or before the requested date (covers weekends/holidays generously).
const fetchWindow = 14

var ErrNoRate = errors.New("no exchange rate available")

// RateStore is the subset of database.Store the converter needs.
type RateStore interface {
	UpsertRate(ctx context.Context, date time.Time, base, quote string, rate float64, source string) error
	GetLatestRateOnOrBefore(ctx context.Context, base, quote string, date time.Time) (float64, time.Time, error)
}

// USDSeriesFetcher fetches date -> symbol -> units-per-USD (Frankfurter).
type USDSeriesFetcher interface {
	FetchUSDSeries(ctx context.Context, start, end time.Time, symbols []string) (map[string]map[string]float64, error)
}

// DayFetcher fetches code -> units-per-USD for one day (NBK).
type DayFetcher interface {
	FetchDay(ctx context.Context, date time.Time) (map[string]float64, error)
}

// Converter converts amounts between currencies using the rate effective on a
// given date (spec §2.2: rate-on-effective-date, never today's rate).
type Converter struct {
	Store RateStore
	Frank USDSeriesFetcher
	NBK   DayFetcher

	mu   sync.Mutex
	memo map[string]memoEntry // request-level cache: "CCY|date" -> rate
}

type memoEntry struct {
	rate float64
	date time.Time
}

func NewConverter(store RateStore, frank USDSeriesFetcher, nbk DayFetcher) *Converter {
	return &Converter{Store: store, Frank: frank, NBK: nbk, memo: map[string]memoEntry{}}
}

// Convert converts amount from -> to using the rate on (or carried forward to)
// date. Returns the converted amount and the actual rate date used.
func (c *Converter) Convert(ctx context.Context, amount float64, from, to string, date time.Time) (float64, time.Time, error) {
	if from == to {
		return amount, date, nil
	}
	fromRate, fromDate, err := c.usdRate(ctx, from, date)
	if err != nil {
		return 0, time.Time{}, err
	}
	toRate, toDate, err := c.usdRate(ctx, to, date)
	if err != nil {
		return 0, time.Time{}, err
	}
	// Cross via USD: amount / (from per USD) * (to per USD).
	converted := amount / fromRate * toRate
	// Report the older of the two rate dates as the effective rate date.
	rateDate := fromDate
	if toDate.Before(rateDate) {
		rateDate = toDate
	}
	if from == "USD" {
		rateDate = toDate
	} else if to == "USD" {
		rateDate = fromDate
	}
	return converted, rateDate, nil
}

// usdRate returns units of ccy per 1 USD on (or carried forward to) date.
func (c *Converter) usdRate(ctx context.Context, ccy string, date time.Time) (float64, time.Time, error) {
	if ccy == "USD" {
		return 1, date, nil
	}
	key := ccy + "|" + date.Format("2006-01-02")
	c.mu.Lock()
	if m, ok := c.memo[key]; ok {
		c.mu.Unlock()
		return m.rate, m.date, nil
	}
	c.mu.Unlock()

	rate, rateDate, err := c.Store.GetLatestRateOnOrBefore(ctx, "USD", ccy, date)
	stale := err == nil && rateDate.Before(date.AddDate(0, 0, -fetchWindow))

	// Cache miss (or suspiciously stale hit): fetch the window from the source.
	if errors.Is(err, database.ErrNotFound) || stale {
		if fetchErr := c.fetchAndCache(ctx, ccy, date); fetchErr != nil {
			log.Printf("rate fetch for %s: %v", ccy, fetchErr)
		}
		rate, rateDate, err = c.Store.GetLatestRateOnOrBefore(ctx, "USD", ccy, date)
	}
	if errors.Is(err, database.ErrNotFound) {
		return 0, time.Time{}, fmt.Errorf("%w for %s on %s", ErrNoRate, ccy, date.Format("2006-01-02"))
	}
	if err != nil {
		return 0, time.Time{}, err
	}

	c.mu.Lock()
	c.memo[key] = memoEntry{rate: rate, date: rateDate}
	c.mu.Unlock()
	return rate, rateDate, nil
}

// fetchAndCache pulls rates for [date-fetchWindow, date] from the right source
// and upserts them into the store.
func (c *Converter) fetchAndCache(ctx context.Context, ccy string, date time.Time) error {
	start := date.AddDate(0, 0, -fetchWindow)

	if FrankfurterCurrencies[ccy] && c.Frank != nil {
		series, err := c.Frank.FetchUSDSeries(ctx, start, date, []string{ccy})
		if err != nil {
			return err
		}
		for dateStr, symbols := range series {
			d, err := time.Parse("2006-01-02", dateStr)
			if err != nil {
				continue
			}
			for symbol, rate := range symbols {
				if err := c.Store.UpsertRate(ctx, d, "USD", symbol, rate, "frankfurter"); err != nil {
					return err
				}
			}
		}
		return nil
	}

	if c.NBK != nil {
		// Walk back from the requested date until NBK has data (max 7 days).
		for i := 0; i <= 7; i++ {
			d := date.AddDate(0, 0, -i)
			day, err := c.NBK.FetchDay(ctx, d)
			if err != nil {
				return err
			}
			if len(day) == 0 {
				continue
			}
			for code, rate := range day {
				if err := c.Store.UpsertRate(ctx, d, "USD", code, rate, "nbk"); err != nil {
					return err
				}
			}
			if _, ok := day[ccy]; ok {
				return nil // found what we came for
			}
		}
	}
	return fmt.Errorf("no source covers currency %s", ccy)
}
