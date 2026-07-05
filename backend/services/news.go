package services

import (
	"context"

	"dynamics-dashboard/database"
)

// NewsStore persists fetched headlines and lists events awaiting one.
// Fetching itself happens in the GitHub Actions workflow (GDELT rate-limits
// cloud egress IPs like Render's; runner IPs are fresh every run) — the
// backend only serves the queue and stores the results.
type NewsStore interface {
	UpdateEventNews(ctx context.Context, id int64, headline, url string) error
	GetEventsMissingNews(ctx context.Context, limit int) ([]database.CurrencyEvent, error)
}

// newsTermsForCurrency maps a currency to GDELT search terms.
var newsTermsForCurrency = map[string]string{
	"KZT": "Kazakhstan tenge", "USD": "US dollar", "EUR": "euro currency",
	"RUB": "Russian ruble", "GBP": "British pound sterling", "CHF": "Swiss franc",
	"JPY": "Japanese yen", "CNY": "Chinese yuan", "TRY": "Turkish lira",
	"AUD": "Australian dollar", "BGN": "Bulgarian lev", "BRL": "Brazilian real",
	"CAD": "Canadian dollar", "CZK": "Czech koruna", "DKK": "Danish krone",
	"HKD": "Hong Kong dollar", "HUF": "Hungarian forint", "IDR": "Indonesian rupiah",
	"ILS": "Israeli shekel", "INR": "Indian rupee", "ISK": "Icelandic krona",
	"KRW": "Korean won", "MXN": "Mexican peso", "MYR": "Malaysian ringgit",
	"NOK": "Norwegian krone", "NZD": "New Zealand dollar", "PHP": "Philippine peso",
	"PLN": "Polish zloty", "RON": "Romanian leu", "SEK": "Swedish krona",
	"SGD": "Singapore dollar", "THB": "Thai baht", "ZAR": "South African rand",
}

// majors are currencies whose pair-partner is usually the story.
var majorCurrencies = map[string]bool{"USD": true, "EUR": true, "GBP": true, "CHF": true, "JPY": true}

// NewsTermsForPair picks the search terms for an event on base/quote:
// prefer the non-major currency (that's where the news is), fall back to base.
func NewsTermsForPair(base, quote string) string {
	pick := base
	if majorCurrencies[base] && !majorCurrencies[quote] {
		pick = quote
	}
	if terms, ok := newsTermsForCurrency[pick]; ok {
		return terms
	}
	return pick + " currency exchange rate"
}
