package services

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// NBK fetches official daily rates from the National Bank of Kazakhstan.
// NBK publishes rates as "KZT per N units of foreign currency"; we normalize
// everything to "units of currency per 1 USD" via the USD/KZT rate, which
// gives us KZT plus ~40 other currencies (including RUB) from one fetch.
type NBK struct {
	BaseURL string
	Client  *http.Client
}

func NewNBK() *NBK {
	return &NBK{
		BaseURL: "https://nationalbank.kz",
		Client:  &http.Client{Timeout: 15 * time.Second},
	}
}

type nbkRates struct {
	Items []nbkItem `xml:"item"`
}

type nbkItem struct {
	Title       string `xml:"title"`       // currency code, e.g. "USD"
	Description string `xml:"description"` // KZT per Quant units
	Quant       string `xml:"quant"`       // units, e.g. "1" or "100"
}

// FetchDay returns code -> (units of code per 1 USD) for the given date,
// including "KZT" itself. Returns an empty map when NBK has no data for the date.
func (n *NBK) FetchDay(ctx context.Context, date time.Time) (map[string]float64, error) {
	url := fmt.Sprintf("%s/rss/get_rates.cfm?fdate=%s", n.BaseURL, date.Format("02.01.2006"))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := n.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nbk request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nbk returned status %d", resp.StatusCode)
	}

	var rates nbkRates
	if err := xml.NewDecoder(resp.Body).Decode(&rates); err != nil {
		return nil, fmt.Errorf("nbk decode: %w", err)
	}

	// KZT per 1 unit of each currency.
	kztPer := map[string]float64{}
	for _, item := range rates.Items {
		code := strings.TrimSpace(item.Title)
		desc, err1 := strconv.ParseFloat(strings.TrimSpace(item.Description), 64)
		quant, err2 := strconv.ParseFloat(strings.TrimSpace(item.Quant), 64)
		if code == "" || err1 != nil || err2 != nil || quant == 0 || desc <= 0 {
			continue
		}
		kztPer[code] = desc / quant
	}

	kztPerUSD, ok := kztPer["USD"]
	if !ok || kztPerUSD <= 0 {
		return map[string]float64{}, nil // no usable data for this date
	}

	// Normalize: units of C per 1 USD = (KZT per USD) / (KZT per C).
	out := map[string]float64{"KZT": kztPerUSD}
	for code, kztRate := range kztPer {
		if code == "USD" {
			continue
		}
		out[code] = kztPerUSD / kztRate
	}
	return out, nil
}
