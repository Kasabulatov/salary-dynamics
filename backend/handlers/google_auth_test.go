package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"dynamics-dashboard/database"
	"dynamics-dashboard/security"
	"dynamics-dashboard/services"
)

type fakeGoogle struct {
	configured bool
	user       services.GoogleUser
	err        error
}

func (f *fakeGoogle) Configured() bool { return f.configured }
func (f *fakeGoogle) AuthorizeURL(_, state string) string {
	return "https://accounts.google.com/o/oauth2/v2/auth?state=" + state
}
func (f *fakeGoogle) ExchangeCodeForUser(_ context.Context, _, _ string) (services.GoogleUser, error) {
	return f.user, f.err
}

type fakeGoogleStore struct {
	byEmail map[string]database.User
	nextID  int64
}

func (f *fakeGoogleStore) FindOrCreateGoogleUser(_ context.Context, email string) (database.User, error) {
	if u, ok := f.byEmail[email]; ok {
		return u, nil // link to existing
	}
	f.nextID++
	u := database.User{ID: f.nextID, Email: email}
	f.byEmail[email] = u
	return u, nil
}

func newGoogleHandler(g *fakeGoogle, store *fakeGoogleStore) *GoogleAuthHandler {
	return &GoogleAuthHandler{
		Store: store, Google: g, JWTSecret: testSecret,
		PublicAPIURL: "http://localhost:8080", FrontendURL: "http://localhost:5173",
	}
}

func emptyStore() *fakeGoogleStore { return &fakeGoogleStore{byEmail: map[string]database.User{}} }

func sessionAuthCookie(w *httptest.ResponseRecorder) string {
	for _, c := range w.Result().Cookies() {
		if c.Name == authCookie && c.Value != "" {
			return c.Value
		}
	}
	return ""
}

func callbackReq(cookieState, queryState, code, errParam string) *http.Request {
	u := "/api/oauth/google/callback?state=" + queryState
	if code != "" {
		u += "&code=" + code
	}
	if errParam != "" {
		u += "&error=" + errParam
	}
	r := httptest.NewRequest(http.MethodGet, u, nil)
	r.AddCookie(&http.Cookie{Name: googleStateCookie, Value: cookieState})
	return r
}

func TestGoogleConfig(t *testing.T) {
	for cfg, want := range map[bool]string{true: `"google":true`, false: `"google":false`} {
		h := newGoogleHandler(&fakeGoogle{configured: cfg}, emptyStore())
		w := httptest.NewRecorder()
		h.Config(w, httptest.NewRequest(http.MethodGet, "/api/auth/config", nil))
		if !strings.Contains(w.Body.String(), want) {
			t.Errorf("config(%v) = %s, want %s", cfg, w.Body, want)
		}
	}
}

func TestGoogleStart(t *testing.T) {
	// Unconfigured → 503.
	w := httptest.NewRecorder()
	newGoogleHandler(&fakeGoogle{configured: false}, emptyStore()).
		Start(w, httptest.NewRequest(http.MethodGet, "/api/oauth/google/start", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("unconfigured start = %d, want 503", w.Code)
	}

	// Configured → 302 to Google, HttpOnly state cookie set.
	w = httptest.NewRecorder()
	newGoogleHandler(&fakeGoogle{configured: true}, emptyStore()).
		Start(w, httptest.NewRequest(http.MethodGet, "/api/oauth/google/start", nil))
	if w.Code != http.StatusFound || !strings.Contains(w.Header().Get("Location"), "accounts.google.com") {
		t.Errorf("configured start = %d -> %s", w.Code, w.Header().Get("Location"))
	}
	var stateSet, httpOnly bool
	for _, c := range w.Result().Cookies() {
		if c.Name == googleStateCookie && c.Value != "" {
			stateSet, httpOnly = true, c.HttpOnly
		}
	}
	if !stateSet || !httpOnly {
		t.Errorf("state cookie set=%v httpOnly=%v, want both true", stateSet, httpOnly)
	}
}

func TestGoogleCallbackStateMismatch(t *testing.T) {
	h := newGoogleHandler(&fakeGoogle{configured: true}, emptyStore())
	w := httptest.NewRecorder()
	h.Callback(w, callbackReq("bbb", "aaa", "code", "")) // cookie != query
	if !strings.Contains(w.Header().Get("Location"), "auth_error=state") {
		t.Errorf("state mismatch -> %s", w.Header().Get("Location"))
	}
	if sessionAuthCookie(w) != "" {
		t.Error("no session must be issued on state mismatch")
	}
}

func TestGoogleCallbackUnverifiedEmail(t *testing.T) {
	g := &fakeGoogle{configured: true, user: services.GoogleUser{Email: "a@gmail.com", EmailVerified: false}}
	h := newGoogleHandler(g, emptyStore())
	w := httptest.NewRecorder()
	h.Callback(w, callbackReq("s", "s", "code", ""))
	if !strings.Contains(w.Header().Get("Location"), "auth_error=unverified") {
		t.Errorf("unverified -> %s", w.Header().Get("Location"))
	}
	if sessionAuthCookie(w) != "" {
		t.Error("no session on unverified email (would allow account takeover)")
	}
}

func TestGoogleCallbackDeclined(t *testing.T) {
	h := newGoogleHandler(&fakeGoogle{configured: true}, emptyStore())
	w := httptest.NewRecorder()
	h.Callback(w, callbackReq("s", "s", "", "access_denied"))
	if !strings.Contains(w.Header().Get("Location"), "auth_error=declined") {
		t.Errorf("declined -> %s", w.Header().Get("Location"))
	}
}

func TestGoogleCallbackLinksExistingByEmail(t *testing.T) {
	// Existing (password) account with the same email → same account, its
	// JWT identity, no duplicate.
	store := &fakeGoogleStore{byEmail: map[string]database.User{
		"alice@gmail.com": {ID: 42, Email: "alice@gmail.com"},
	}, nextID: 42}
	g := &fakeGoogle{configured: true, user: services.GoogleUser{Email: "alice@gmail.com", EmailVerified: true}}
	h := newGoogleHandler(g, store)
	w := httptest.NewRecorder()
	h.Callback(w, callbackReq("s", "s", "code", ""))

	if w.Code != http.StatusFound || !strings.HasSuffix(w.Header().Get("Location"), ":5173/") {
		t.Errorf("callback = %d -> %s, want 302 to frontend root", w.Code, w.Header().Get("Location"))
	}
	token := sessionAuthCookie(w)
	if token == "" {
		t.Fatal("session not issued")
	}
	id, err := security.ParseToken(testSecret, token)
	if err != nil || id != 42 {
		t.Errorf("session user = %d (err %v), want 42 (linked, not a new account)", id, err)
	}
	if len(store.byEmail) != 1 {
		t.Errorf("account count = %d, want 1 (no duplicate created)", len(store.byEmail))
	}
}

func TestGoogleCallbackCreatesNewUser(t *testing.T) {
	store := emptyStore()
	g := &fakeGoogle{configured: true, user: services.GoogleUser{Email: "new@gmail.com", EmailVerified: true}}
	h := newGoogleHandler(g, store)
	w := httptest.NewRecorder()
	h.Callback(w, callbackReq("s", "s", "code", ""))

	if _, ok := store.byEmail["new@gmail.com"]; !ok {
		t.Error("new Google user was not created")
	}
	if sessionAuthCookie(w) == "" {
		t.Error("session not issued for the new user")
	}
}
