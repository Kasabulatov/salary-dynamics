package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GDELT fetches news headlines from the GDELT DOC 2.0 API — free, no key,
// global archive back to 2017. Used to explain ⚡ exchange-rate events.
type GDELT struct {
	BaseURL string
	Client  *http.Client
}

func NewGDELT() *GDELT {
	return &GDELT{
		BaseURL: "https://api.gdeltproject.org",
		Client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// Headline is one news article reference.
type Headline struct {
	Title string
	URL   string
}

type gdeltResponse struct {
	Articles []struct {
		URL   string `json:"url"`
		Title string `json:"title"`
	} `json:"articles"`
}

// FetchTopHeadline returns the most relevant English headline for the query
// terms within [from, to], or nil when nothing was found.
func (g *GDELT) FetchTopHeadline(ctx context.Context, terms string, from, to time.Time) (*Headline, error) {
	q := url.Values{}
	q.Set("query", terms+" sourcelang:eng")
	q.Set("mode", "artlist")
	q.Set("format", "json")
	q.Set("maxrecords", "3")
	q.Set("sort", "hybridrel")
	q.Set("startdatetime", from.Format("20060102")+"000000")
	q.Set("enddatetime", to.Format("20060102")+"235959")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		g.BaseURL+"/api/v2/doc/doc?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "salary-dynamics/1.0 (personal finance dashboard)")
	resp, err := g.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gdelt request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gdelt returned status %d", resp.StatusCode)
	}
	// GDELT returns text/html error pages for malformed queries.
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "json") {
		return nil, fmt.Errorf("gdelt returned non-JSON response (%s)", ct)
	}

	var out gdeltResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("gdelt decode: %w", err)
	}
	for _, a := range out.Articles {
		title := strings.TrimSpace(a.Title)
		if title == "" || a.URL == "" {
			continue
		}
		if len(title) > 200 {
			title = title[:197] + "…"
		}
		return &Headline{Title: title, URL: a.URL}, nil
	}
	return nil, nil // nothing found — a valid outcome
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
