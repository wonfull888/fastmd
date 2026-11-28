package app

import (
	"errors"
	"net/url"

	"github.com/labstack/echo/v4"
	"github.com/wonfull888/fastmd/internal/store"
)

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
		return renderAuth(c, 200, "device", authPageData{Title: "Connect terminal", Login: p.User.GitHubLogin, Code: c.QueryParam("user_code"), CSRF: p.CSRF})
	})
	e.POST("/auth/device", func(c echo.Context) error {
		// HTML forms carry their CSRF token in the form body.
		c.Request().Header.Set("X-CSRF-Token", c.FormValue("csrf_token"))
		return a.withSession(func(c echo.Context, p store.Credential) error {
			allowed := c.FormValue("decision") == "allow"
			if err := a.DB.DecideDevice(c.FormValue("user_code"), p.User.ID, allowed); err != nil {
				if !errors.Is(err, store.ErrUnauthorized) {
					return a.fail(c, err)
				}
				return renderAuth(c, 400, "device", authPageData{Title: "Connect terminal", Login: p.User.GitHubLogin, Code: c.FormValue("user_code"), CSRF: p.CSRF,
					Error: "This code is invalid, expired, or already used. Start a new login in your terminal."})
			}
			return renderDeviceResult(c, allowed)
		})(c)
	})
}
