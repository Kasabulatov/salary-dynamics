package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeConverter converts via a fixed units-per-USD rate table (like the real
// USD-cross converter, minus the database and external sources).
type fakeConverter struct {
	perUSD map[string]float64 // e.g. "KZT": 450 means 450 KZT per USD
}

func (f *fakeConverter) Convert(_ context.Context, amount float64, from, to string, date time.Time) (float64, time.Time, error) {
	if from == to {
		return amount, date, nil
	}
	fromRate, ok1 := f.rate(from)
	toRate, ok2 := f.rate(to)
	if !ok1 || !ok2 {
		return 0, time.Time{}, fmt.Errorf("no rate for pair %s/%s", from, to)
	}
	return amount / fromRate * toRate, date, nil
}

func (f *fakeConverter) rate(ccy string) (float64, bool) {
	if ccy == "USD" {
		return 1, true
	}
	r, ok := f.perUSD[ccy]
	return r, ok
}

func postCompare(t *testing.T, h *PublicHandler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/public/compare", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Compare(w, req)
	return w
}

func decodeCompare(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v; body: %s", err, w.Body)
	}
	return out
}

// --- unit: the verdict/percent phrasing ---

func TestPctPhrase(t *testing.T) {
	cases := []struct {
		pct  float64
		want string
	}{
		{25.4, "about 25% more"},
		{125.0, "about 125% more"},
		{-8.2, "about 8% less"},
		{0.4, "about the same"},
		{-0.9, "about the same"},
	}
	for _, tc := range cases {
		if got := pctPhrase(tc.pct); got != tc.want {
			t.Errorf("pctPhrase(%v) = %q, want %q", tc.pct, got, tc.want)
		}
	}
}

func TestBuildVerdict(t *testing.T) {
	v := buildVerdict(125, 39.1, true)
	if !strings.Contains(v, "about 125% more by exchange rate") ||
		!strings.Contains(v, "about 39% more in real purchasing power") {
		t.Errorf("verdict = %q, want both comparisons", v)
	}

	v = buildVerdict(-10, 0, false)
	if !strings.Contains(v, "about 10% less by exchange rate") ||
		!strings.Contains(v, "could not be compared") {
		t.Errorf("unavailable verdict = %q", v)
	}
}

// --- integration: the endpoint with a fake converter ---

func testCompareHandler() *PublicHandler {
	return &PublicHandler{Converter: &fakeConverter{perUSD: map[string]float64{
		"KZT": 500, "EUR": 0.9,
	}}}
}

func TestCompareHappyPathWithCOL(t *testing.T) {
	// Same converted value on both sides (10_000 USD each), different cities:
	// Almaty (COL 34) vs Lisbon (COL 55) → FX 0%, real ≈ -38.2% (Lisbon is
	// pricier, the same money buys less).
	h := testCompareHandler()
	w := postCompare(t, h, `{
		"current": {"amount": 10000, "currency": "USD", "city": "Almaty"},
		"offer":   {"amount": 10000, "currency": "USD", "city": "Lisbon"},
		"displayCurrency": "USD"
	}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body: %s", w.Code, w.Body)
	}
	out := decodeCompare(t, w)

	fx := out["fx"].(map[string]any)["offerVsCurrentPct"].(float64)
	if fx != 0 {
		t.Errorf("fx pct = %v, want 0 (same converted value)", fx)
	}
	real := out["real"].(map[string]any)
	if real["available"] != true {
		t.Fatalf("real.available = %v, want true", real["available"])
	}
	realPct := real["offerVsCurrentPct"].(float64)
	if realPct < -38.3 || realPct > -38.1 {
		t.Errorf("real pct = %v, want ~-38.2 (COL 34 -> 55)", realPct)
	}
	verdict := out["verdict"].(string)
	if !strings.Contains(verdict, "about the same by exchange rate") ||
		!strings.Contains(verdict, "about 38% less in real purchasing power") {
		t.Errorf("verdict = %q", verdict)
	}
}

func TestCompareCrossCurrency(t *testing.T) {
	// 500_000 KZT (rate 500) = 1000 USD vs 1800 EUR (rate 0.9) = 2000 USD
	// → FX +100%.
	h := testCompareHandler()
	w := postCompare(t, h, `{
		"current": {"amount": 500000, "currency": "KZT", "city": "Almaty"},
		"offer":   {"amount": 1800, "currency": "EUR", "city": "Lisbon"}
	}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body: %s", w.Code, w.Body)
	}
	out := decodeCompare(t, w)
	if out["displayCurrency"] != "USD" {
		t.Errorf("default displayCurrency = %v, want USD", out["displayCurrency"])
	}
	fx := out["fx"].(map[string]any)["offerVsCurrentPct"].(float64)
	if fx != 100 {
		t.Errorf("fx pct = %v, want 100", fx)
	}
	// Real: 1000/(0.34)=2941.2 vs 2000/(0.55)=3636.4 → +23.6%.
	realPct := out["real"].(map[string]any)["offerVsCurrentPct"].(float64)
	if realPct < 23.5 || realPct > 23.8 {
		t.Errorf("real pct = %v, want ~23.6", realPct)
	}
}

func TestCompareUnknownCityDegradesToFXOnly(t *testing.T) {
	// EUR maps to the euro-area code XC, which has no country row: an
	// unknown EUR city means no COL → FX-only with the explanatory note.
	h := testCompareHandler()
	w := postCompare(t, h, `{
		"current": {"amount": 1000, "currency": "USD", "city": "New York"},
		"offer":   {"amount": 2000, "currency": "EUR", "city": "Atlantis"}
	}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body: %s", w.Code, w.Body)
	}
	out := decodeCompare(t, w)
	real := out["real"].(map[string]any)
	if real["available"] != false {
		t.Fatalf("real.available = %v, want false", real["available"])
	}
	offer := out["offer"].(map[string]any)
	if offer["realValue"] != nil || offer["colIndex"] != nil {
		t.Errorf("offer real fields = %v/%v, want null", offer["realValue"], offer["colIndex"])
	}
	if !strings.Contains(out["verdict"].(string), "could not be compared") {
		t.Errorf("verdict = %q, want unavailable phrasing", out["verdict"])
	}
	notes := fmt.Sprintf("%v", out["notes"])
	if !strings.Contains(notes, "unavailable for this location") {
		t.Errorf("notes = %s, want the FX-only note", notes)
	}
}

func TestCompareCountryFallbackNote(t *testing.T) {
	// Unknown KZT city falls back to the KZ country index, with a note.
	h := testCompareHandler()
	w := postCompare(t, h, `{
		"current": {"amount": 500000, "currency": "KZT", "city": "Karaganda"},
		"offer":   {"amount": 1000, "currency": "USD", "city": "New York"}
	}`)
	out := decodeCompare(t, w)
	current := out["current"].(map[string]any)
	if current["colLevel"] != "country" {
		t.Errorf("colLevel = %v, want country", current["colLevel"])
	}
	if !strings.Contains(fmt.Sprintf("%v", out["notes"]), "Country-level") {
		t.Errorf("notes missing country-level disclaimer: %v", out["notes"])
	}
}

func TestCompareValidation(t *testing.T) {
	h := testCompareHandler()
	cases := []struct {
		name string
		body string
	}{
		{"zero amount", `{"current":{"amount":0,"currency":"USD"},"offer":{"amount":1,"currency":"USD"}}`},
		{"huge amount", `{"current":{"amount":1e13,"currency":"USD"},"offer":{"amount":1,"currency":"USD"}}`},
		{"bad currency", `{"current":{"amount":1,"currency":"DOLLARS"},"offer":{"amount":1,"currency":"USD"}}`},
		{"bad display", `{"current":{"amount":1,"currency":"USD"},"offer":{"amount":1,"currency":"USD"},"displayCurrency":"US"}`},
		{"unsupported currency", `{"current":{"amount":1,"currency":"XYZ"},"offer":{"amount":1,"currency":"USD"}}`},
		{"not json", `hello`},
	}
	for _, tc := range cases {
		if w := postCompare(t, h, tc.body); w.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400; body: %s", tc.name, w.Code, w.Body)
		}
	}
}

func TestCompareMeta(t *testing.T) {
	h := &PublicHandler{}
	req := httptest.NewRequest(http.MethodGet, "/api/public/compare/meta", nil)
	w := httptest.NewRecorder()
	h.CompareMeta(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{"Almaty", "Lisbon", `"updated"`, "source_note"} {
		if !strings.Contains(body, want) {
			t.Errorf("meta missing %s", want)
		}
	}
}
