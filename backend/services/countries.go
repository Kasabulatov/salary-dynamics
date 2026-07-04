package services

// countryForCurrency maps a salary currency to the World Bank country code
// whose CPI applies ("XC" is the World Bank code for the euro area).
var countryForCurrency = map[string]string{
	"USD": "US", "EUR": "XC", "KZT": "KZ", "RUB": "RU", "GBP": "GB",
	"CHF": "CH", "JPY": "JP", "CNY": "CN", "TRY": "TR", "AUD": "AU",
	"BGN": "BG", "BRL": "BR", "CAD": "CA", "CZK": "CZ", "DKK": "DK",
	"HKD": "HK", "HUF": "HU", "IDR": "ID", "ILS": "IL", "INR": "IN",
	"ISK": "IS", "KRW": "KR", "MXN": "MX", "MYR": "MY", "NOK": "NO",
	"NZD": "NZ", "PHP": "PH", "PLN": "PL", "RON": "RO", "SEK": "SE",
	"SGD": "SG", "THB": "TH", "ZAR": "ZA",
}

// CountryForCurrency returns the World Bank country code for a currency.
func CountryForCurrency(ccy string) (string, bool) {
	c, ok := countryForCurrency[ccy]
	return c, ok
}
