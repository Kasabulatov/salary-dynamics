package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GoogleUser is the identity we need from Google sign-in.
type GoogleUser struct {
	Sub           string
	Email         string
	EmailVerified bool
}

// GoogleAuthenticator is the surface the auth handler depends on (real impl
// below; a fake is used in tests so no network is touched).
type GoogleAuthenticator interface {
	Configured() bool
	AuthorizeURL(redirectURI, state string) string
	ExchangeCodeForUser(ctx context.Context, code, redirectURI string) (GoogleUser, error)
}

// Google implements OAuth 2.0 / OpenID Connect against Google's endpoints.
type Google struct {
	AuthURL      string // https://accounts.google.com/o/oauth2/v2/auth
	TokenURL     string // https://oauth2.googleapis.com/token
	UserInfoURL  string // https://openidconnect.googleapis.com/v1/userinfo
	ClientID     string
	ClientSecret string
	Client       *http.Client
}

func NewGoogle(clientID, clientSecret string) *Google {
	return &Google{
		AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:     "https://oauth2.googleapis.com/token",
		UserInfoURL:  "https://openidconnect.googleapis.com/v1/userinfo",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Client:       &http.Client{Timeout: 15 * time.Second},
	}
}

func (g *Google) Configured() bool { return g.ClientID != "" && g.ClientSecret != "" }

// AuthorizeURL builds the consent-screen redirect. Non-sensitive scopes only.
func (g *Google) AuthorizeURL(redirectURI, state string) string {
	q := url.Values{}
	q.Set("client_id", g.ClientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", "openid email profile")
	q.Set("state", state)
	q.Set("access_type", "online")
	q.Set("prompt", "select_account")
	return g.AuthURL + "?" + q.Encode()
}

// ExchangeCodeForUser swaps the authorization code for tokens, then reads the
// verified identity from the userinfo endpoint.
func (g *Google) ExchangeCodeForUser(ctx context.Context, code, redirectURI string) (GoogleUser, error) {
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", g.ClientID)
	form.Set("client_secret", g.ClientSecret)
	form.Set("redirect_uri", redirectURI)
	form.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return GoogleUser{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := g.Client.Do(req)
	if err != nil {
		return GoogleUser{}, fmt.Errorf("google token request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return GoogleUser{}, fmt.Errorf("google token endpoint returned %d", resp.StatusCode)
	}
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return GoogleUser{}, fmt.Errorf("google token decode: %w", err)
	}
	if tok.AccessToken == "" {
		return GoogleUser{}, fmt.Errorf("google token response missing access_token")
	}

	ureq, err := http.NewRequestWithContext(ctx, http.MethodGet, g.UserInfoURL, nil)
	if err != nil {
		return GoogleUser{}, err
	}
	ureq.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	uresp, err := g.Client.Do(ureq)
	if err != nil {
		return GoogleUser{}, fmt.Errorf("google userinfo request: %w", err)
	}
	defer uresp.Body.Close()
	if uresp.StatusCode != http.StatusOK {
		return GoogleUser{}, fmt.Errorf("google userinfo returned %d", uresp.StatusCode)
	}
	var info struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	}
	if err := json.NewDecoder(uresp.Body).Decode(&info); err != nil {
		return GoogleUser{}, fmt.Errorf("google userinfo decode: %w", err)
	}
	return GoogleUser{Sub: info.Sub, Email: strings.ToLower(strings.TrimSpace(info.Email)), EmailVerified: info.EmailVerified}, nil
}
