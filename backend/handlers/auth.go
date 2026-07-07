package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/mail"
	"time"

	"dynamics-dashboard/database"
	"dynamics-dashboard/security"
)

const (
	authCookie = "auth"
	csrfCookie = "csrf"
	tokenTTL   = 24 * time.Hour
)

// UserStore is the subset of database.Store the auth handlers need.
type UserStore interface {
	CreateUser(ctx context.Context, email, passwordHash string) (database.User, error)
	GetUserByEmail(ctx context.Context, email string) (database.User, error)
	GetUserByID(ctx context.Context, id int64) (database.User, error)
}

type AuthHandler struct {
	Store          UserStore
	JWTSecret      []byte
	CookieSecure   bool
	CookieSameSite http.SameSite
}

func (h *AuthHandler) sameSite() http.SameSite {
	if h.CookieSameSite == 0 {
		return http.SameSiteLaxMode
	}
	return h.CookieSameSite
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if _, err := mail.ParseAddress(c.Email); err != nil {
		writeErr(w, http.StatusBadRequest, "please enter a valid email address")
		return
	}
	if len(c.Password) < 8 {
		writeErr(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	if len(c.Password) > 72 { // bcrypt silently truncates beyond 72 bytes
		writeErr(w, http.StatusBadRequest, "password must be at most 72 characters")
		return
	}

	hash, err := security.HashPassword(c.Password)
	if err != nil {
		log.Printf("hash password: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	u, err := h.Store.CreateUser(r.Context(), c.Email, hash)
	if errors.Is(err, database.ErrEmailTaken) {
		writeErr(w, http.StatusConflict, "email already registered")
		return
	}
	if err != nil {
		log.Printf("create user: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, u)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	u, err := h.Store.GetUserByEmail(r.Context(), c.Email)
	if errors.Is(err, database.ErrNotFound) || (err == nil && !security.CheckPasswordHash(c.Password, u.PasswordHash)) {
		writeErr(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	if err != nil {
		log.Printf("get user: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	token, err := security.IssueToken(h.JWTSecret, u.ID, tokenTTL)
	if err != nil {
		log.Printf("issue token: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	csrf, err := security.NewCSRFToken()
	if err != nil {
		log.Printf("csrf token: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	setSessionCookies(w, token, csrf, h.CookieSecure, h.sameSite())
	writeJSON(w, http.StatusOK, u)
}

// setSessionCookies writes the HttpOnly JWT cookie (not readable by JS —
// XSS-safe) plus the JS-readable CSRF cookie the frontend echoes in
// X-CSRF-Token. Shared by password login and Google sign-in so both issue an
// identical session.
func setSessionCookies(w http.ResponseWriter, token, csrf string, secure bool, sameSite http.SameSite) {
	http.SetCookie(w, &http.Cookie{
		Name: authCookie, Value: token, Path: "/",
		MaxAge: int(tokenTTL.Seconds()), HttpOnly: true,
		Secure: secure, SameSite: sameSite,
	})
	http.SetCookie(w, &http.Cookie{
		Name: csrfCookie, Value: csrf, Path: "/",
		MaxAge: int(tokenTTL.Seconds()), HttpOnly: false,
		Secure: secure, SameSite: sameSite,
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	for _, name := range []string{authCookie, csrfCookie} {
		http.SetCookie(w, &http.Cookie{
			Name: name, Value: "", Path: "/", MaxAge: -1,
			HttpOnly: name == authCookie,
			Secure:   h.CookieSecure, SameSite: h.sameSite(),
		})
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}

// Me returns the currently authenticated user (for session restoration).
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFrom(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	u, err := h.Store.GetUserByID(r.Context(), userID)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	writeJSON(w, http.StatusOK, u)
}
