package database

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// UpsertRate inserts or updates one daily rate row.
func (s *Store) UpsertRate(ctx context.Context, date time.Time, base, quote string, rate float64, source string) error {
	_, err := s.Pool.Exec(ctx,
		`INSERT INTO daily_rates (date, base_currency, quote_currency, rate, source)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (date, base_currency, quote_currency)
		 DO UPDATE SET rate = EXCLUDED.rate, source = EXCLUDED.source`,
		date, base, quote, rate, source)
	return err
}

// GetLatestRateOnOrBefore returns the most recent rate for (base, quote) at or
// before date — the "last known rate" carry-forward lookup.
func (s *Store) GetLatestRateOnOrBefore(ctx context.Context, base, quote string, date time.Time) (float64, time.Time, error) {
	var rate float64
	var rateDate time.Time
	err := s.Pool.QueryRow(ctx,
		`SELECT rate, date FROM daily_rates
		 WHERE base_currency = $1 AND quote_currency = $2 AND date <= $3
		 ORDER BY date DESC LIMIT 1`,
		base, quote, date).Scan(&rate, &rateDate)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, time.Time{}, ErrNotFound
	}
	return rate, rateDate, err
}

// CountRates returns the number of cached rate rows (used by the refresh endpoint).
func (s *Store) CountRates(ctx context.Context) (int64, error) {
	var n int64
	err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM daily_rates`).Scan(&n)
	return n, err
}

type RatePoint struct {
	Date time.Time
	Rate float64
}

// GetRatesForCurrency returns all USD->quote rates in [from, to], ascending.
func (s *Store) GetRatesForCurrency(ctx context.Context, quote string, from, to time.Time) ([]RatePoint, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT date, rate FROM daily_rates
		 WHERE base_currency = 'USD' AND quote_currency = $1 AND date BETWEEN $2 AND $3
		 ORDER BY date ASC`, quote, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []RatePoint
	for rows.Next() {
		var p RatePoint
		if err := rows.Scan(&p.Date, &p.Rate); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	return points, rows.Err()
}

// GetRateCoverage reports the min/max rate dates and row count for a currency
// within [from, to] — used to decide whether a backfill fetch is needed.
func (s *Store) GetRateCoverage(ctx context.Context, quote string, from, to time.Time) (time.Time, time.Time, int64, error) {
	var minD, maxD *time.Time
	var count int64
	err := s.Pool.QueryRow(ctx,
		`SELECT min(date), max(date), count(*) FROM daily_rates
		 WHERE base_currency = 'USD' AND quote_currency = $1 AND date BETWEEN $2 AND $3`,
		quote, from, to).Scan(&minD, &maxD, &count)
	if err != nil || count == 0 {
		return time.Time{}, time.Time{}, 0, err
	}
	return *minD, *maxD, count, nil
}
