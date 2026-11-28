package app

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/wonfull888/fastmd/internal/service"
	"github.com/wonfull888/fastmd/internal/store"
)

type GitHubConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	// Endpoint overrides are only used by isolated provider tests.
	AuthorizeURL string
	TokenURL     string
	UserURL      string
	HTTPClient   *http.Client
}

func (a *App) registerGitHub(e *echo.Echo) {
	g := &a.Options.GitHub
	if g.AuthorizeURL == "" {
		g.AuthorizeURL = "https://github.com/login/oauth/authorize"
	}
	if g.TokenURL == "" {
		g.TokenURL = "https://github.com/login/oauth/access_token"
	}
	if g.UserURL == "" {
		g.UserURL = "https://api.github.com/user"
	}
	if g.RedirectURI == "" {
		g.RedirectURI = a.Options.PublicURL + "/auth/github/callback"
	}
	if g.HTTPClient == nil {
		g.HTTPClient = &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	e.GET("/auth/github/start", a.githubStart)
	e.GET("/auth/github/callback", a.githubCallback)
}
func (a *App) githubStart(c echo.Context) error {
	c.Response().Header().Set("Cache-Control", "no-store")
	g := a.Options.GitHub
	if g.ClientID == "" || g.ClientSecret == "" {
		return a.githubPageError(c, 503, "LOGIN_NOT_CONFIGURED", "Sign-in is temporarily unavailable. Please try again later.", "/dashboard")
	}
	state, err := store.RandomSecret("")
	if err != nil {
		return a.fail(c, err)
	}
	verifier, err := store.RandomSecret("")
	if err != nil {
		return a.fail(c, err)
	}
	if err := a.DB.BeginGitHub(state, verifier, safeReturn(c.QueryParam("next"))); err != nil {
		return a.fail(c, err)
	}
	c.SetCookie(&http.Cookie{Name: "fastmd_github_state", Value: state, Path: "/auth/github", MaxAge: 600, HttpOnly: true, Secure: strings.HasPrefix(a.Options.PublicURL, "https://"), SameSite: http.SameSiteLaxMode})
	digest := sha256.Sum256([]byte(verifier))
	values := url.Values{"client_id": {g.ClientID}, "redirect_uri": {g.RedirectURI}, "state": {state}, "scope": {""}, "code_challenge_method": {"S256"}, "code_challenge": {base64.RawURLEncoding.EncodeToString(digest[:])}}
	return c.Redirect(http.StatusFound, g.AuthorizeURL+"?"+values.Encode())
}
func (a *App) githubCallback(c echo.Context) error {
	c.Response().Header().Set("Cache-Control", "no-store")
	c.Response().Header().Set("Referrer-Policy", "no-referrer")
	cookie, err := c.Cookie("fastmd_github_state")
	state := c.QueryParam("state")
	if err != nil || state == "" || subtle.ConstantTimeCompare([]byte(state), []byte(cookie.Value)) != 1 {
		return a.githubPageError(c, 400, "INVALID_STATE", "This sign-in expired or was not started in this browser. Please start again.", "/dashboard")
	}
	c.SetCookie(&http.Cookie{Name: "fastmd_github_state", Value: "", Path: "/auth/github", MaxAge: -1, HttpOnly: true, Secure: strings.HasPrefix(a.Options.PublicURL, "https://"), SameSite: http.SameSiteLaxMode})
	verifier, next, err := a.DB.ConsumeGitHub(state)
	if err != nil {
		return a.githubPageError(c, 400, "INVALID_STATE", "This sign-in expired or was already used. Please start again.", "/dashboard")
	}
	if c.QueryParam("error") != "" || c.QueryParam("code") == "" {
		return a.githubPageError(c, 400, "LOGIN_CANCELLED", "GitHub sign-in was cancelled. You can try again when you are ready.", next)
	}
	profile, err := a.githubIdentity(c.Request().Context(), c.QueryParam("code"), verifier)
	if err != nil {
		return a.githubPageError(c, 502, "GITHUB_UNAVAILABLE", "GitHub sign-in could not be completed. Please try again.", next)
	}
	user, err := a.DB.GitHubLogin(profile.ID, profile.Login)
	if err != nil {
		return a.fail(c, err)
	}
	raw, _, err := a.DB.IssueCredential(user.ID, "session", "GitHub login", "read write delete account", time.Now().Unix()+30*86400)
	if err != nil {
		return a.fail(c, err)
	}
	c.SetCookie(&http.Cookie{Name: "fastmd_session", Value: raw, Path: "/", MaxAge: 30 * 86400, HttpOnly: true, Secure: strings.HasPrefix(a.Options.PublicURL, "https://"), SameSite: http.SameSiteLaxMode})
	c.SetCookie(&http.Cookie{Name: "fastmd_token", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	return c.Redirect(http.StatusSeeOther, safeReturn(next))
}

type githubProfile struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}

func (a *App) githubIdentity(ctx context.Context, code, verifier string) (githubProfile, error) {
	g := a.Options.GitHub
	form := url.Values{"client_id": {g.ClientID}, "client_secret": {g.ClientSecret}, "code": {code}, "redirect_uri": {g.RedirectURI}, "code_verifier": {verifier}}
	req, err := http.NewRequestWithContext(ctx, "POST", g.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return githubProfile{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := g.HTTPClient.Do(req)
	if err != nil {
		return githubProfile{}, err
	}
	var token struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	err = json.NewDecoder(io.LimitReader(response.Body, 64*1024)).Decode(&token)
	response.Body.Close()
	if err != nil || response.StatusCode != 200 || token.AccessToken == "" || token.Error != "" {
		return githubProfile{}, errors.New("GitHub exchange failed")
	}
	req, err = http.NewRequestWithContext(ctx, "GET", g.UserURL, nil)
	if err != nil {
		return githubProfile{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "fastmd")
	response, err = g.HTTPClient.Do(req)
	if err != nil {
		return githubProfile{}, err
	}
	defer response.Body.Close()
	var profile githubProfile
	err = json.NewDecoder(io.LimitReader(response.Body, 64*1024)).Decode(&profile)
	if err != nil || response.StatusCode != 200 || profile.ID <= 0 || profile.Login == "" {
		return profile, errors.New("GitHub profile failed")
	}
	// GitHub tokens are used only to fetch identity and are never persisted.
	return profile, nil
}

func (a *App) githubPageError(c echo.Context, status int, code, message, next string) error {
	if strings.Contains(c.Request().Header.Get("Accept"), "text/html") {
		return renderAuth(c, status, "error", authPageData{Title: "Unable to sign in", Message: message,
			Retry: "/login?next=" + url.QueryEscape(safeReturn(next))})
	}
	return a.fail(c, service.Failure(code, message, status))
}
