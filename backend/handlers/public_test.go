package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// postCompute hits Compute with nil deps: validation must reject bad input
// before any dependency is touched (nil deps would panic otherwise).
func postCompute(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	h := &PublicHandler{}
	req := httptest.NewRequest(http.MethodPost, "/api/public/compute", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Compute(w, req)
	return w
}

func TestPublicComputeValidation(t *testing.T) {
	tooMany := `{"entries":[`
	for i := 0; i < 51; i++ {
		if i > 0 {
			tooMany += ","
		}
		tooMany += fmt.Sprintf(`{"amount":100,"currency_code":"USD","effective_date":"2024-01-%02d"}`, i%28+1)
	}
	tooMany += `]}`

	cases := []struct {
		name string
		body string
		want int
	}{
		{"empty entries", `{"entries":[]}`, http.StatusBadRequest},
		{"missing entries", `{}`, http.StatusBadRequest},
		{"51 entries", tooMany, http.StatusBadRequest},
		{"bad currency", `{"entries":[{"amount":100,"currency_code":"usd!","effective_date":"2024-01-01"}]}`, http.StatusBadRequest},
		{"zero amount", `{"entries":[{"amount":0,"currency_code":"USD","effective_date":"2024-01-01"}]}`, http.StatusBadRequest},
		{"bad date", `{"entries":[{"amount":100,"currency_code":"USD","effective_date":"01/01/2024"}]}`, http.StatusBadRequest},
		{"bad display currency", `{"entries":[{"amount":100,"currency_code":"USD","effective_date":"2024-01-01"}],"display_currency":"DOLLARS"}`, http.StatusBadRequest},
		{"bad from", `{"entries":[{"amount":100,"currency_code":"USD","effective_date":"2024-01-01"}],"from":"yesterday"}`, http.StatusBadRequest},
		{"not json", `hello`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		w := postCompute(t, tc.body)
		if w.Code != tc.want {
			t.Errorf("%s: status = %d, want %d; body: %s", tc.name, w.Code, tc.want, w.Body)
		}
	}
}
