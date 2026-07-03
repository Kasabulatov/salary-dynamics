package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"dynamics-dashboard/database"
	"dynamics-dashboard/security"
)

// fakeUserStore is an in-memory UserStore for handler tests.
type fakeUserStore struct {
	users  map[string]database.User
	nextID int64
}

func newFakeUserStore() *fakeUserStore {
	return &fakeUserStore{users: map[string]database.User{}, nextID: 1}
}

func (f *fakeUserStore) CreateUser(_ context.Context, email, hash string) (database.User, error) {
	if _, ok := f.users[email]; ok {
		return database.User{}, database.ErrEmailTaken
	}
	u := database.User{ID: f.nextID, Email: email, PasswordHash: hash, DefaultDisplayCurrency: "USD"}
	f.nextID++
	f.users[email] = u
	return u, nil
}

func (f *fakeUserStore) GetUserByEmail(_ context.Context, email string) (database.User, error) {
	u, ok := f.users[email]
	if !ok {
		return database.User{}, database.ErrNotFound
	}
	return u, nil
}

func (f *fakeUserStore) GetUserByID(_ context.Context, id int64) (database.User, error) {
	for _, u := range f.users {
		if u.ID == id {
			return u, nil
		}
	}
	return database.User{}, database.ErrNotFound
}

var testSecret = []byte("test-jwt-secret")

func newTestAuthHandler() *AuthHandler {
	return &AuthHandler{Store: newFakeUserStore(), JWTSecret: testSecret}
}

func post(h http.HandlerFunc, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h(w, req)
	return w
}

func TestRegister(t *testing.T) {
	h := newTestAuthHandler()

	w := post(h.Register, `{"email":"a@example.com","password":"password123"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", w.Code, w.Body)
	}
	if strings.Contains(w.Body.String(), "password") {
		t.Error("response leaks password field")
	}

	// Duplicate email → 409.
	w = post(h.Register, `{"email":"a@example.com","password":"password123"}`)
	if w.Code != http.StatusConflict {
		t.Errorf("duplicate: status = %d, want 409", w.Code)
	}

	// Invalid email → 400.
	w = post(h.Register, `{"email":"not-an-email","password":"password123"}`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("bad email: status = %d, want 400", w.Code)
	}

	// Short password → 400.
	w = post(h.Register, `{"email":"b@example.com","password":"short"}`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("short password: status = %d, want 400", w.Code)
	}
}

func TestLogin(t *testing.T) {
	h := newTestAuthHandler()
	post(h.Register, `{"email":"a@example.com","password":"password123"}`)

	// Wrong password → 401.
	w := post(h.Login, `{"email":"a@example.com","password":"wrongpass"}`)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("wrong password: status = %d, want 401", w.Code)
	}

	// Unknown user → 401 (same message, no user enumeration).
	w = post(h.Login, `{"email":"nobody@example.com","password":"password123"}`)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("unknown user: status = %d, want 401", w.Code)
	}

	// Correct credentials → 200 + auth & csrf cookies.
	w = post(h.Login, `{"email":"a@example.com","password":"password123"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("login: status = %d, want 200; body: %s", w.Code, w.Body)
	}
	cookies := w.Result().Cookies()
	var gotAuth, gotCSRF bool
	for _, c := range cookies {
		switch c.Name {
		case "auth":
			gotAuth = true
			if !c.HttpOnly {
				t.Error("auth cookie must be HttpOnly")
			}
		case "csrf":
			gotCSRF = true
			if c.HttpOnly {
				t.Error("csrf cookie must NOT be HttpOnly (JS reads it)")
			}
		}
	}
	if !gotAuth || !gotCSRF {
		t.Errorf("cookies set: auth=%v csrf=%v, want both", gotAuth, gotCSRF)
	}
}

func TestAuthMiddleware(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, found := UserIDFrom(r.Context()); !found {
			t.Error("user ID missing from context")
		}
		w.WriteHeader(http.StatusOK)
	})
	protected := Auth(testSecret)(ok)

	// No cookie → 401.
	w := httptest.NewRecorder()
	protected.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusUnauthorized {
		t.Errorf("no cookie: status = %d, want 401", w.Code)
	}

	// Invalid token → 401.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "auth", Value: "garbage"})
	w = httptest.NewRecorder()
	protected.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("bad token: status = %d, want 401", w.Code)
	}

	// Valid token → 200.
	token, _ := security.IssueToken(testSecret, 7, time.Hour)
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})
	w = httptest.NewRecorder()
	protected.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("valid token: status = %d, want 200", w.Code)
	}
}

func TestCSRFMiddleware(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	protected := CSRF(ok)

	// GET passes without a token.
	w := httptest.NewRecorder()
	protected.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusOK {
		t.Errorf("GET: status = %d, want 200", w.Code)
	}

	// POST without token → 403.
	w = httptest.NewRecorder()
	protected.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/", nil))
	if w.Code != http.StatusForbidden {
		t.Errorf("POST no token: status = %d, want 403", w.Code)
	}

	// POST with mismatched header → 403.
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.AddCookie(&http.Cookie{Name: "csrf", Value: "aaa"})
	req.Header.Set("X-CSRF-Token", "bbb")
	w = httptest.NewRecorder()
	protected.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("POST mismatch: status = %d, want 403", w.Code)
	}

	// POST with matching cookie + header → 200.
	req = httptest.NewRequest(http.MethodPost, "/", nil)
	req.AddCookie(&http.Cookie{Name: "csrf", Value: "match-token"})
	req.Header.Set("X-CSRF-Token", "match-token")
	w = httptest.NewRecorder()
	protected.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("POST match: status = %d, want 200", w.Code)
	}
}
