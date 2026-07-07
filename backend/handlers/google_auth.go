package handlers

import (
	"context"
	"crypto/subtle"
	"log"
	"net/http"
	"strings"

	"dynamics-dashboard/database"
	"dynamics-dashboard/security"
	"dynamics-dashboard/services"
)

const googleStateCookie = "g_oauth_state"

// GoogleStore is the subset of database.Store Google sign-in needs.
type GoogleStore interface {
	FindOrCreateGoogleUser(ctx context.Context, email string) (database.User, error)
}

// GoogleAuthHandler runs "Sign in with Google": it verifies the email with
// Google, links it to an account by email (existing account preserved,
// password intact), and issues the SAME session cookies as password login.
type GoogleAuthHandler struct {
	Store          GoogleStore
	Google         services.GoogleAuthenticator
	JWTSecret      []byte
	CookieSecure   bool
	CookieSameSite http.SameSite
	PublicAPIURL   string // backend base — builds the OAuth redirect_uri
	FrontendURL    string // where to send the browser after login (CORS origin)
}

func (h *GoogleAuthHandler) redirectURI() string {
	return strings.TrimSuffix(h.PublicAPIURL, "/") + "/api/oauth/google/callback"
}

func (h *GoogleAuthHandler) sameSite() http.SameSite {
	if h.CookieSameSite == 0 {
		return http.SameSiteLaxMode
	}
	return h.CookieSameSite
}

// Config: GET /api/auth/config — tells the frontend whether to show the
// "Continue with Google" button (hidden when unconfigured).
func (h *GoogleAuthHandler) Config(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"google": h.Google.Configured()})
}

// Start: GET /api/oauth/google/start — redirect to Google's consent screen,
// with a random state stored in a short-lived cookie (login-CSRF guard).
func (h *GoogleAuthHandler) Start(w http.ResponseWriter, r *http.Request) {
	if !h.Google.Configured() {
		writeErr(w, http.StatusServiceUnavailable, "Google sign-in is not configured")
		return
	}
	state, err := security.NewCSRFToken()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: googleStateCookie, Value: state, Path: "/api/oauth",
		MaxAge: 600, HttpOnly: true, Secure: h.CookieSecure, SameSite: h.sameSite(),
	})
	http.Redirect(w, r, h.Google.AuthorizeURL(h.redirectURI(), state), http.StatusFound)
}

// Callback: GET /api/oauth/google/callback — verify state, exchange the code,
// require a VERIFIED email, find-or-create the account, issue the session,
// and send the browser back to the app. Failures redirect to /login with an
// auth_error the login page can explain.
func (h *GoogleAuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	frontend := strings.TrimSuffix(h.FrontendURL, "/") + "/"
	redirectErr := func(code string) {
		http.Redirect(w, r, frontend+"login?auth_error="+code, http.StatusFound)
	}
	// Clear the state cookie regardless of outcome.
	defer http.SetCookie(w, &http.Cookie{
		Name: googleStateCookie, Value: "", Path: "/api/oauth", MaxAge: -1,
		HttpOnly: true, Secure: h.CookieSecure, SameSite: h.sameSite(),
	})

	if !h.Google.Configured() {
		redirectErr("not_configured")
		return
	}
	stateCookie, err := r.Cookie(googleStateCookie)
	state := r.URL.Query().Get("state")
	if err != nil || state == "" ||
		subtle.ConstantTimeCompare([]byte(stateCookie.Value), []byte(state)) != 1 {
		redirectErr("state")
		return
	}
	if r.URL.Query().Get("error") != "" {
		redirectErr("declined") // user cancelled on Google's screen
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		redirectErr("no_code")
		return
	}

	gu, err := h.Google.ExchangeCodeForUser(r.Context(), code, h.redirectURI())
	if err != nil {
		log.Printf("google callback exchange: %v", err)
		redirectErr("exchange")
		return
	}
	if gu.Email == "" || !gu.EmailVerified {
		redirectErr("unverified") // never link/create on an unverified email
		return
	}

	u, err := h.Store.FindOrCreateGoogleUser(r.Context(), gu.Email)
	if err != nil {
		log.Printf("google callback find-or-create: %v", err)
		redirectErr("internal")
		return
	}
	token, err := security.IssueToken(h.JWTSecret, u.ID, tokenTTL)
	if err != nil {
		redirectErr("internal")
		return
	}
	csrf, err := security.NewCSRFToken()
	if err != nil {
		redirectErr("internal")
		return
	}
	setSessionCookies(w, token, csrf, h.CookieSecure, h.sameSite())
	http.Redirect(w, r, frontend, http.StatusFound)
}
