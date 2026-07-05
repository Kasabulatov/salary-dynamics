package services

import "testing"

func TestNewsTermsForPair(t *testing.T) {
	cases := []struct{ base, quote, want string }{
		{"KZT", "USD", "Kazakhstan tenge"}, // non-major side wins
		{"USD", "KZT", "Kazakhstan tenge"},
		{"EUR", "TRY", "Turkish lira"},
		{"KZT", "EUR", "Kazakhstan tenge"},
		{"USD", "EUR", "US dollar"}, // both major -> base
	}
	for _, tc := range cases {
		if got := NewsTermsForPair(tc.base, tc.quote); got != tc.want {
			t.Errorf("NewsTermsForPair(%s, %s) = %q, want %q", tc.base, tc.quote, got, tc.want)
		}
	}
}
