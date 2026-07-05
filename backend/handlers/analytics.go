package handlers

import (
	"context"
	"crypto/subtle"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"dynamics-dashboard/database"
	"dynamics-dashboard/security"
	"dynamics-dashboard/services"
)

const oauthStateCookie = "oauth_state"

// AnalyticsStore is the subset of database.Store the analytics module needs.
type AnalyticsStore interface {
	UpsertAnalyticsConnection(ctx context.Context, userID int64, provider, encAccess, encRefresh string, expiresAt *time.Time) (database.AnalyticsConnection, error)
	GetAnalyticsConnection(ctx context.Context, userID int64, provider string) (database.AnalyticsConnection, error)
	ListAnalyticsConnections(ctx context.Context, userID int64) ([]database.AnalyticsConnection, error)
	DeleteAnalyticsConnection(ctx context.Context, userID int64, provider string) error
}

// AnalyticsHandler wires OAuth + Metrica data for the Product Metrics module.
type AnalyticsHandler struct {
	Store         AnalyticsStore
	Yandex        *services.Yandex
	EncryptionKey []byte // nil when TOKEN_ENCRYPTION_KEY is unset
	PublicAPIURL  string // OAuth callback base (this backend's public URL)
	FrontendURL   string // where to send the browser after the callback
	CookieSecure  bool
}

func (h *AnalyticsHandler) configured() bool {
	return h.Yandex != nil && h.Yandex.Configured() && len(h.EncryptionKey) == 32
}

func (h *AnalyticsHandler) redirectURI() string {
	return strings.TrimSuffix(h.PublicAPIURL, "/") + "/api/oauth/yandex/callback"
}

// OAuthStart redirects the browser to Yandex's consent screen. A random
// state value is stored in a short-lived HttpOnly cookie and must round-trip.
func (h *AnalyticsHandler) OAuthStart(w http.ResponseWriter, r *http.Request) {
	if !h.configured() {
		writeErr(w, http.StatusServiceUnavailable,
			"analytics is not configured (YANDEX_CLIENT_ID / YANDEX_CLIENT_SECRET / TOKEN_ENCRYPTION_KEY)")
		return
	}
	state, err := security.NewCSRFToken()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: oauthStateCookie, Value: state, Path: "/api/oauth",
		MaxAge: 300, HttpOnly: true, Secure: h.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, h.Yandex.AuthorizeURL(h.redirectURI(), state), http.StatusFound)
}

// OAuthCallback exchanges the code, encrypts the tokens, stores the
// connection, and sends the browser back to the frontend metrics page.
func (h *AnalyticsHandler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserIDFrom(r.Context())
	frontend := strings.TrimSuffix(h.FrontendURL, "/") + "/metrics"

	if !h.configured() {
		http.Redirect(w, r, frontend+"?error=not_configured", http.StatusFound)
		return
	}

	// CSRF: state must match the cookie set at start.
	stateCookie, err := r.Cookie(oauthStateCookie)
	state := r.URL.Query().Get("state")
	if err != nil || state == "" ||
		subtle.ConstantTimeCompare([]byte(stateCookie.Value), []byte(state)) != 1 {
		http.Redirect(w, r, frontend+"?error=state_mismatch", http.StatusFound)
		return
	}
	// Clear the state cookie either way.
	http.SetCookie(w, &http.Cookie{Name: oauthStateCookie, Value: "", Path: "/api/oauth",
		MaxAge: -1, HttpOnly: true, Secure: h.CookieSecure, SameSite: http.SameSiteLaxMode})

	if errCode := r.URL.Query().Get("error"); errCode != "" {
		http.Redirect(w, r, frontend+"?error="+errCode, http.StatusFound) // user declined
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Redirect(w, r, frontend+"?error=missing_code", http.StatusFound)
		return
	}

	token, err := h.Yandex.ExchangeCode(r.Context(), code)
	if err != nil {
		log.Printf("oauth yandex exchange: %v", err)
		http.Redirect(w, r, frontend+"?error=exchange_failed", http.StatusFound)
		return
	}

	encAccess, err := security.EncryptString(h.EncryptionKey, token.AccessToken)
	if err != nil {
		log.Printf("oauth encrypt: %v", err)
		http.Redirect(w, r, frontend+"?error=internal", http.StatusFound)
		return
	}
	encRefresh, err := security.EncryptString(h.EncryptionKey, token.RefreshToken)
	if err != nil {
		log.Printf("oauth encrypt: %v", err)
		http.Redirect(w, r, frontend+"?error=internal", http.StatusFound)
		return
	}
	var expiresAt *time.Time
	if token.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
		expiresAt = &t
	}
	if _, err := h.Store.UpsertAnalyticsConnection(r.Context(), userID, "yandex", encAccess, encRefresh, expiresAt); err != nil {
		log.Printf("oauth store connection: %v", err)
		http.Redirect(w, r, frontend+"?error=internal", http.StatusFound)
		return
	}
	http.Redirect(w, r, frontend+"?connected=yandex", http.StatusFound)
}

// Connections lists the user's provider connections (tokens never leave).
func (h *AnalyticsHandler) Connections(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserIDFrom(r.Context())
	conns, err := h.Store.ListAnalyticsConnections(r.Context(), userID)
	if err != nil {
		log.Printf("analytics connections: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"connections": conns,
		"configured":  h.configured(),
	})
}

// Disconnect removes a provider connection.
func (h *AnalyticsHandler) Disconnect(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserIDFrom(r.Context())
	provider := r.URL.Query().Get("provider")
	if provider != "yandex" {
		writeErr(w, http.StatusBadRequest, "provider must be 'yandex'")
		return
	}
	if err := h.Store.DeleteAnalyticsConnection(r.Context(), userID, provider); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "no such connection")
			return
		}
		log.Printf("analytics disconnect: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "disconnected"})
}

// accessToken decrypts the stored token, refreshing it once if rejected.
func (h *AnalyticsHandler) accessToken(ctx context.Context, userID int64) (string, error) {
	conn, err := h.Store.GetAnalyticsConnection(ctx, userID, "yandex")
	if err != nil {
		return "", err
	}
	return security.DecryptString(h.EncryptionKey, conn.EncryptedAccessToken)
}

// refreshAndStore refreshes the token after a rejection and returns the new one.
func (h *AnalyticsHandler) refreshAndStore(ctx context.Context, userID int64) (string, error) {
	conn, err := h.Store.GetAnalyticsConnection(ctx, userID, "yandex")
	if err != nil {
		return "", err
	}
	refresh, err := security.DecryptString(h.EncryptionKey, conn.EncryptedRefreshToken)
	if err != nil || refresh == "" {
		return "", services.ErrTokenExpired
	}
	token, err := h.Yandex.RefreshToken(ctx, refresh)
	if err != nil {
		return "", err
	}
	encAccess, err := security.EncryptString(h.EncryptionKey, token.AccessToken)
	if err != nil {
		return "", err
	}
	encRefresh := conn.EncryptedRefreshToken
	if token.RefreshToken != "" {
		if encRefresh, err = security.EncryptString(h.EncryptionKey, token.RefreshToken); err != nil {
			return "", err
		}
	}
	var expiresAt *time.Time
	if token.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
		expiresAt = &t
	}
	if _, err := h.Store.UpsertAnalyticsConnection(ctx, userID, "yandex", encAccess, encRefresh, expiresAt); err != nil {
		return "", err
	}
	return token.AccessToken, nil
}

// withToken runs fn with a valid access token, refreshing once on rejection.
func (h *AnalyticsHandler) withToken(ctx context.Context, userID int64, fn func(token string) error) error {
	token, err := h.accessToken(ctx, userID)
	if err != nil {
		return err
	}
	err = fn(token)
	if errors.Is(err, services.ErrTokenExpired) {
		token, err = h.refreshAndStore(ctx, userID)
		if err != nil {
			return err
		}
		return fn(token)
	}
	return err
}

// Counters lists the user's Metrica counters.
func (h *AnalyticsHandler) Counters(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserIDFrom(r.Context())
	var counters []services.Counter
	err := h.withToken(r.Context(), userID, func(token string) error {
		var e error
		counters, e = h.Yandex.ListCounters(r.Context(), token)
		return e
	})
	if errors.Is(err, database.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "no Yandex connection — connect your account first")
		return
	}
	if errors.Is(err, services.ErrTokenExpired) {
		writeErr(w, http.StatusUnauthorized, "Yandex session expired — please reconnect")
		return
	}
	if err != nil {
		log.Printf("analytics counters: %v", err)
		writeErr(w, http.StatusBadGateway, "could not reach Yandex Metrica")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"counters": counters})
}

// Data returns daily users/sessions/pageviews for a counter and range.
func (h *AnalyticsHandler) Data(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserIDFrom(r.Context())

	counterID, err := strconv.ParseInt(r.URL.Query().Get("counter"), 10, 64)
	if err != nil || counterID <= 0 {
		writeErr(w, http.StatusBadRequest, "counter must be a Metrica counter ID")
		return
	}
	now := time.Now().UTC().Truncate(24 * time.Hour)
	from := now.AddDate(0, -1, 0)
	to := now
	if s := r.URL.Query().Get("from"); s != "" {
		d, err := time.Parse("2006-01-02", s)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "from must be YYYY-MM-DD")
			return
		}
		from = d
	}
	if s := r.URL.Query().Get("to"); s != "" {
		d, err := time.Parse("2006-01-02", s)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "to must be YYYY-MM-DD")
			return
		}
		to = d
	}

	var points []services.MetricsPoint
	err = h.withToken(r.Context(), userID, func(token string) error {
		var e error
		points, e = h.Yandex.FetchDailyMetrics(r.Context(), token, counterID, from, to)
		return e
	})
	if errors.Is(err, database.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "no Yandex connection — connect your account first")
		return
	}
	if errors.Is(err, services.ErrTokenExpired) {
		writeErr(w, http.StatusUnauthorized, "Yandex session expired — please reconnect")
		return
	}
	if err != nil {
		log.Printf("analytics data: %v", err)
		writeErr(w, http.StatusBadGateway, "could not reach Yandex Metrica")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"points": points})
}
