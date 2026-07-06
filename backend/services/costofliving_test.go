package services

import "testing"

func TestLookupCOLExactCity(t *testing.T) {
	index, matched, level := LookupCOL("Almaty", "KZ")
	if !matched || level != "city" {
		t.Fatalf("Almaty: matched=%v level=%s, want city match", matched, level)
	}
	if index != 34 {
		t.Errorf("Almaty index = %v, want 34 (from committed dataset)", index)
	}
}

func TestLookupCOLNormalizesCaseAndSpaces(t *testing.T) {
	cases := []string{"almaty", "ALMATY", "  Almaty  ", "saint   petersburg", "NEW YORK"}
	for _, city := range cases {
		if _, matched, level := LookupCOL(city, ""); !matched || level != "city" {
			t.Errorf("%q: matched=%v level=%s, want city match", city, matched, level)
		}
	}
}

func TestLookupCOLCountryFallback(t *testing.T) {
	// Unknown city, known country (derived from currency by the caller).
	index, matched, level := LookupCOL("Karaganda", "KZ")
	if !matched || level != "country" {
		t.Fatalf("Karaganda/KZ: matched=%v level=%s, want country fallback", matched, level)
	}
	if index != 33 {
		t.Errorf("KZ country index = %v, want 33", index)
	}

	// Empty city also falls back to country.
	if _, matched, level := LookupCOL("", "PT"); !matched || level != "country" {
		t.Errorf("empty city/PT: matched=%v level=%s, want country fallback", matched, level)
	}
}

func TestLookupCOLNone(t *testing.T) {
	// Unknown city + unmapped country (e.g. the euro area code XC has no
	// country row) → none: caller degrades to FX-only comparison.
	if _, matched, level := LookupCOL("Atlantis", "XC"); matched || level != "none" {
		t.Errorf("Atlantis/XC: matched=%v level=%s, want none", matched, level)
	}
	if _, matched, level := LookupCOL("", ""); matched || level != "none" {
		t.Errorf("empty/empty: matched=%v level=%s, want none", matched, level)
	}
}

func TestCOLMetadata(t *testing.T) {
	if COLUpdated() == "" {
		t.Error("dataset updated date missing (UI disclaimer needs it)")
	}
	if len(COLCities()) < 30 {
		t.Errorf("dataset has %d cities, want the seeded ~36", len(COLCities()))
	}
	if COLSourceNote() == "" {
		t.Error("source note missing")
	}
}
