package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strings"
	"time"

	"dynamics-dashboard/services"
)

// Offer Comparison ("What-If"): guest, stateless, rate-limited like
// /api/public/compute. Uses TODAY'S rate (latest cached + carry-forward via
// the existing Converter) — this is a forward-looking decision, unlike the
// salary tracker's rate-on-effective-date pipeline, which stays untouched.

type compareSide struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
	City     string  `json:"city"`
}

type compareRequest struct {
	Current         compareSide `json:"current"`
	Offer           compareSide `json:"offer"`
	DisplayCurrency string      `json:"displayCurrency"`
}

type compareSideResult struct {
	AmountConverted float64  `json:"amountConverted"`
	COLIndex        *float64 `json:"colIndex"`  // nil when no COL match
	RealValue       *float64 `json:"realValue"` // nil when no COL match
	COLLevel        string   `json:"colLevel"`  // "city" | "country" | "none"
}

func (s *compareSide) validate(label string) string {
	if s.Amount <= 0 {
		return label + ": amount must be greater than 0"
	}
	if s.Amount > 1e12 {
		return label + ": amount is implausibly large"
	}
	s.Currency = strings.ToUpper(strings.TrimSpace(s.Currency))
	if !currencyRe.MatchString(s.Currency) {
		return label + ": currency must be a 3-letter ISO 4217 code"
	}
	s.City = strings.TrimSpace(s.City)
	if len(s.City) > 100 {
		return label + ": city name is too long"
	}
	return ""
}

// resolveCOL: exact city → country (derived from the salary currency) → none.
func resolveCOL(side compareSide) (index float64, matched bool, level string) {
	country, _ := services.CountryForCurrency(side.Currency)
	return services.LookupCOL(side.City, country)
}

// pctPhrase turns +25.3 into "about 25% more", -8.2 into "about 8% less",
// and |pct| < 1 into "about the same".
func pctPhrase(pct float64) string {
	if math.Abs(pct) < 1 {
		return "about the same"
	}
	direction := "more"
	if pct < 0 {
		direction = "less"
	}
	return fmt.Sprintf("about %.0f%% %s", math.Abs(pct), direction)
}

// buildVerdict produces the plain-language summary. The server owns this
// string so the page and the share image always say the same thing.
func buildVerdict(fxPct float64, realPct float64, realAvailable bool) string {
	if !realAvailable {
		return fmt.Sprintf(
			"The offer is worth %s by exchange rate. Cost-of-living data is unavailable for this location, so real purchasing power could not be compared.",
			pctPhrase(fxPct))
	}
	return fmt.Sprintf(
		"The offer is worth %s by exchange rate, and %s in real purchasing power.",
		pctPhrase(fxPct), pctPhrase(realPct))
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }

// Compare handles POST /api/public/compare.
func (h *PublicHandler) Compare(w http.ResponseWriter, r *http.Request) {
	if h.Converter == nil {
		writeErr(w, http.StatusServiceUnavailable, "comparison is not available right now")
		return
	}

	var req compareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if msg := req.Current.validate("current"); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	if msg := req.Offer.validate("offer"); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	display := strings.ToUpper(strings.TrimSpace(req.DisplayCurrency))
	if display == "" {
		display = "USD"
	}
	if !currencyRe.MatchString(display) {
		writeErr(w, http.StatusBadRequest, "displayCurrency must be a 3-letter ISO 4217 code")
		return
	}

	// Today's rate: latest cached row + last-known carry-forward + the same
	// on-demand fetch the tracker uses. No effective-date semantics.
	today := time.Now().UTC().Truncate(24 * time.Hour)
	convert := func(side compareSide, label string) (float64, bool) {
		converted, _, err := h.Converter.Convert(r.Context(), side.Amount, side.Currency, display, today)
		if err != nil {
			log.Printf("compare: convert %s: %v", label, err)
			writeErr(w, http.StatusBadRequest,
				fmt.Sprintf("no exchange rate available for %s — check the currency code", side.Currency))
			return 0, false
		}
		return converted, true
	}
	currentConv, ok := convert(req.Current, "current")
	if !ok {
		return
	}
	offerConv, ok := convert(req.Offer, "offer")
	if !ok {
		return
	}

	buildSide := func(side compareSide, converted float64) compareSideResult {
		res := compareSideResult{AmountConverted: round1(converted), COLLevel: "none"}
		if index, matched, level := resolveCOL(side); matched {
			real := round1(converted / (index / 100))
			res.COLIndex, res.RealValue, res.COLLevel = &index, &real, level
		}
		return res
	}
	current := buildSide(req.Current, currentConv)
	offer := buildSide(req.Offer, offerConv)

	fxPct := round1((offerConv/currentConv - 1) * 100)
	realAvailable := current.RealValue != nil && offer.RealValue != nil
	realPct := 0.0
	if realAvailable {
		realPct = round1((*offer.RealValue / *current.RealValue - 1) * 100)
	}

	notes := []string{
		"Exchange rates are today's rates.",
		fmt.Sprintf("Cost-of-living indices are approximate, manually curated (updated %s).", services.COLUpdated()),
	}
	if !realAvailable {
		notes = append(notes, "Cost-of-living data unavailable for this location — showing exchange-rate comparison only.")
	} else if current.COLLevel == "country" || offer.COLLevel == "country" {
		notes = append(notes, "Country-level cost-of-living used where the exact city is not in the dataset.")
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"displayCurrency": display,
		"current":         current,
		"offer":           offer,
		"fx":              map[string]any{"offerVsCurrentPct": fxPct},
		"real":            map[string]any{"offerVsCurrentPct": realPct, "available": realAvailable},
		"verdict":         buildVerdict(fxPct, realPct, realAvailable),
		"notes":           notes,
	})
}

// CompareMeta handles GET /api/public/compare/meta: the known-cities list
// (for the input datalist) and the dataset disclaimer/date for the UI.
func (h *PublicHandler) CompareMeta(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"cities":      services.COLCities(),
		"updated":     services.COLUpdated(),
		"source_note": services.COLSourceNote(),
	})
}
