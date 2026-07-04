package database

import "context"

// UpsertInflationRate stores one country-year annual CPI rate.
func (s *Store) UpsertInflationRate(ctx context.Context, country string, year int, ratePercent float64, source string) error {
	_, err := s.Pool.Exec(ctx,
		`INSERT INTO inflation_rates (country_code, year, rate_percent, source)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (country_code, year)
		 DO UPDATE SET rate_percent = EXCLUDED.rate_percent, source = EXCLUDED.source`,
		country, year, ratePercent, source)
	return err
}

// GetInflationRates returns year -> rate percent for a country.
func (s *Store) GetInflationRates(ctx context.Context, country string) (map[int]float64, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT year, rate_percent FROM inflation_rates WHERE country_code = $1`, country)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[int]float64{}
	for rows.Next() {
		var year int
		var rate float64
		if err := rows.Scan(&year, &rate); err != nil {
			return nil, err
		}
		out[year] = rate
	}
	return out, rows.Err()
}

// GetDistinctSalaryCurrencies lists every currency present in salary entries
// (used by the daily refresh job to keep inflation data current).
func (s *Store) GetDistinctSalaryCurrencies(ctx context.Context) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `SELECT DISTINCT currency_code FROM salary_entries`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
