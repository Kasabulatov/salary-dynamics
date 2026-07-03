package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// FrankfurterCurrencies is the set of currencies covered by the ECB reference
// rates that Frankfurter serves. (RUB was suspended by the ECB in March 2022;
// it is covered by the NBK source instead.)
var FrankfurterCurrencies = map[string]bool{
	"AUD": true, "BGN": true, "BRL": true, "CAD": true, "CHF": true,
	"CNY": true, "CZK": true, "DKK": true, "EUR": true, "GBP": true,
	"HKD": true, "HUF": true, "IDR": true, "ILS": true, "INR": true,
	"ISK": true, "JPY": true, "KRW": true, "MXN": true, "MYR": true,
	"NOK": true, "NZD": true, "PHP": true, "PLN": true, "RON": true,
	"SEK": true, "SGD": true, "THB": true, "TRY": true, "USD": true,
	"ZAR": true,
}

// Frankfurter fetches historical ECB rates from frankfurter.dev (free, no key).
type Frankfurter struct {
	BaseURL string
	Client  *http.Client
}

func NewFrankfurter() *Frankfurter {
	return &Frankfurter{
		BaseURL: "https://api.frankfurter.dev",
		Client:  &http.Client{Timeout: 15 * time.Second},
	}
}

type frankfurterSeries struct {
	Base  string                        `json:"base"`
	Rates map[string]map[string]float64 `json:"rates"` // date -> symbol -> rate
}

// FetchUSDSeries returns date -> symbol -> (units of symbol per 1 USD) for the
// inclusive range. Empty symbols means all available currencies.
func (f *Frankfurter) FetchUSDSeries(ctx context.Context, start, end time.Time, symbols []string) (map[string]map[string]float64, error) {
	url := fmt.Sprintf("%s/v1/%s..%s?base=USD", f.BaseURL,
		start.Format("2006-01-02"), end.Format("2006-01-02"))
	if len(symbols) > 0 {
		url += "&symbols=" + strings.Join(symbols, ",")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("frankfurter request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("frankfurter returned status %d", resp.StatusCode)
	}

	var series frankfurterSeries
	if err := json.NewDecoder(resp.Body).Decode(&series); err != nil {
		return nil, fmt.Errorf("frankfurter decode: %w", err)
	}
	return series.Rates, nil
}
