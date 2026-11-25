package app

import (
	"errors"
	"html/template"
	"net/http"
	"net/url"

	"github.com/labstack/echo/v4"
	"github.com/wonfull888/fastmd/internal/store"
)

var devicePage = template.Must(template.New("device").Parse(`<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Connect CLI · fastmd</title><link rel="stylesheet" href="/static/style.css"></head><body><main class="container"><section class="card" style="max-width:600px;margin:4rem auto;padding:2rem"><h1>Connect fastmd CLI</h1><p>Signed in as {{.Login}}. Only approve a code shown by a CLI you started.</p><form method="post"><label>CLI code <input name="user_code" value="{{.Code}}" required maxlength="9" autocomplete="off"></label><input type="hidden" name="csrf_token" value="{{.CSRF}}"><button name="decision" value="allow">Connect</button><button name="decision" value="deny">Cancel</button></form></section></main></body></html>`))

func (a *App) registerDevice(e *echo.Echo) {
	e.POST("/v1/auth/device", func(c echo.Context) error {
		c.Response().Header().Set("Cache-Control", "no-store")
		if a.Options.GitHub.ClientID == "" || a.Options.GitHub.ClientSecret == "" {
			return oauthError(c, 503, "login_not_configured", "GitHub login is not configured")
		}
		code, user, err := a.DB.CreateDevice()
		if err != nil {
			return a.fail(c, err)
		}
		uri := a.Options.PublicURL + "/auth/device"
		return c.JSON(200, map[string]any{"device_code": code, "user_code": user, "verification_uri": uri, "verification_uri_complete": uri + "?user_code=" + url.QueryEscape(user), "interval": 5, "expires_in": 600})
	})
	e.POST("/v1/auth/device/token", func(c echo.Context) error {
		c.Response().Header().Set("Cache-Control", "no-store")
		var input struct {
			DeviceCode string `json:"device_code"`
		}
		if err := c.Bind(&input); err != nil {
			return oauthError(c, 400, "invalid_request", "device_code is required")
		}
		token, user, err := a.DB.PollDevice(input.DeviceCode)
		if errors.Is(err, store.ErrPending) {
			return oauthError(c, 400, "authorization_pending", "approve the CLI code in your browser")
		}
		if errors.Is(err, store.ErrSlowDown) {
			return oauthError(c, 400, "slow_down", "wait before polling again")
		}
		if err != nil {
			return oauthError(c, 400, "expired_token", "code expired, was denied or was already used")
		}
		return c.JSON(200, map[string]any{"token": token, "account": user})
	})
	e.GET("/auth/device", func(c echo.Context) error {
		c.Response().Header().Set("Cache-Control", "no-store")
		p, err := a.Session(c)
		if err != nil {
			return a.LoginPage(c, "/auth/device?user_code="+url.QueryEscape(c.QueryParam("user_code")))
		}
		c.Response().Header().Set("Content-Type", "text/html; charset=utf-8")
		return devicePage.Execute(c.Response(), map[string]string{"Login": p.User.GitHubLogin, "Code": c.QueryParam("user_code"), "CSRF": p.CSRF})
	})
	e.POST("/auth/device", func(c echo.Context) error {
		// HTML forms carry their CSRF token in the form body.
		c.Request().Header.Set("X-CSRF-Token", c.FormValue("csrf_token"))
		return a.withSession(func(c echo.Context, p store.Credential) error {
			if err := a.DB.DecideDevice(c.FormValue("user_code"), p.User.ID, c.FormValue("decision") == "allow"); err != nil {
				return oauthError(c, 400, "expired_token", "code expired or was already used")
			}
			c.Response().Header().Set("Content-Type", "text/html; charset=utf-8")
			return c.String(http.StatusOK, "CLI authorization completed. You can close this page and return to your terminal.")
		})(c)
	})
}
