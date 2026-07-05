package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ErrTokenExpired signals the stored access token was rejected — the caller
// should refresh and retry once.
var ErrTokenExpired = errors.New("provider token expired")

// Yandex implements OAuth code exchange and the Metrica read APIs.
// Docs: yandex.ru/dev/id (OAuth), yandex.ru/dev/metrika (Metrica).
type Yandex struct {
	OAuthBaseURL   string // https://oauth.yandex.com
	MetrikaBaseURL string // https://api-metrika.yandex.net
	ClientID       string
	ClientSecret   string
	Client         *http.Client
}

func NewYandex(clientID, clientSecret string) *Yandex {
	return &Yandex{
		OAuthBaseURL:   "https://oauth.yandex.com",
		MetrikaBaseURL: "https://api-metrika.yandex.net",
		ClientID:       clientID,
		ClientSecret:   clientSecret,
		Client:         &http.Client{Timeout: 15 * time.Second},
	}
}

// Configured reports whether OAuth credentials are present.
func (y *Yandex) Configured() bool { return y.ClientID != "" && y.ClientSecret != "" }

// AuthorizeURL builds the user-consent redirect.
func (y *Yandex) AuthorizeURL(redirectURI, state string) string {
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", y.ClientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("state", state)
	return y.OAuthBaseURL + "/authorize?" + q.Encode()
}

// OAuthToken is the result of a code exchange or refresh.
type OAuthToken struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

func (y *Yandex) token(ctx context.Context, form url.Values) (OAuthToken, error) {
	form.Set("client_id", y.ClientID)
	form.Set("client_secret", y.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		y.OAuthBaseURL+"/token", strings.NewReader(form.Encode()))
	if err != nil {
		return OAuthToken{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := y.Client.Do(req)
	if err != nil {
		return OAuthToken{}, fmt.Errorf("yandex token request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return OAuthToken{}, fmt.Errorf("yandex token endpoint returned %d", resp.StatusCode)
	}

	var t OAuthToken
	if err := json.NewDecoder(resp.Body).Decode(&t); err != nil {
		return OAuthToken{}, fmt.Errorf("yandex token decode: %w", err)
	}
	if t.AccessToken == "" {
		return OAuthToken{}, fmt.Errorf("yandex token response missing access_token")
	}
	return t, nil
}

// ExchangeCode swaps an authorization code for tokens.
func (y *Yandex) ExchangeCode(ctx context.Context, code string) (OAuthToken, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	return y.token(ctx, form)
}

// RefreshToken obtains a fresh access token.
func (y *Yandex) RefreshToken(ctx context.Context, refreshToken string) (OAuthToken, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	return y.token(ctx, form)
}

// Counter is one Metrica counter (a tracked site).
type Counter struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Site string `json:"site"`
}

// ListCounters returns the counters the token can read.
func (y *Yandex) ListCounters(ctx context.Context, accessToken string) ([]Counter, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		y.MetrikaBaseURL+"/management/v1/counters", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "OAuth "+accessToken)

	resp, err := y.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("metrica counters request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, ErrTokenExpired
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("metrica counters returned %d", resp.StatusCode)
	}

	var out struct {
		Counters []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
			Site string `json:"site2,omitempty"` // newer field
			Old  string `json:"site,omitempty"`  // legacy field
		} `json:"counters"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("metrica counters decode: %w", err)
	}
	counters := make([]Counter, 0, len(out.Counters))
	for _, c := range out.Counters {
		site := c.Site
		if site == "" {
			site = c.Old
		}
		counters = append(counters, Counter{ID: c.ID, Name: c.Name, Site: site})
	}
	return counters, nil
}

// MetricsPoint is one day of traffic metrics, normalized across providers.
type MetricsPoint struct {
	Date      string  `json:"date"`
	Users     float64 `json:"users"`
	Sessions  float64 `json:"sessions"`
	Pageviews float64 `json:"pageviews"`
}

// FetchDailyMetrics returns users/sessions/pageviews per day for a counter.
func (y *Yandex) FetchDailyMetrics(ctx context.Context, accessToken string, counterID int64, from, to time.Time) ([]MetricsPoint, error) {
	q := url.Values{}
	q.Set("ids", strconv.FormatInt(counterID, 10))
	q.Set("metrics", "ym:s:users,ym:s:visits,ym:s:pageviews")
	q.Set("dimensions", "ym:s:date")
	q.Set("date1", from.Format("2006-01-02"))
	q.Set("date2", to.Format("2006-01-02"))
	q.Set("sort", "ym:s:date")
	q.Set("limit", "1000")
	q.Set("accuracy", "full")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		y.MetrikaBaseURL+"/stat/v1/data?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "OAuth "+accessToken)

	resp, err := y.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("metrica data request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, ErrTokenExpired
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("metrica data returned %d", resp.StatusCode)
	}

	var out struct {
		Data []struct {
			Dimensions []struct {
				Name string `json:"name"`
			} `json:"dimensions"`
			Metrics []float64 `json:"metrics"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("metrica data decode: %w", err)
	}

	points := make([]MetricsPoint, 0, len(out.Data))
	for _, row := range out.Data {
		if len(row.Dimensions) == 0 || len(row.Metrics) < 3 {
			continue
		}
		points = append(points, MetricsPoint{
			Date:      row.Dimensions[0].Name,
			Users:     row.Metrics[0],
			Sessions:  row.Metrics[1],
			Pageviews: row.Metrics[2],
		})
	}
	return points, nil
}
