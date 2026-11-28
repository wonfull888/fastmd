package app

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/wonfull888/fastmd/internal/service"
	"github.com/wonfull888/fastmd/internal/store"
)

type Options struct {
	PublicURL  string
	GitHub     GitHubConfig
	MCPOrigins []string
}
type App struct {
	DB      *store.Store
	Docs    *service.Documents
	Options Options
}

func Register(e *echo.Echo, db *store.Store, options Options) (*App, error) {
	options.PublicURL = strings.TrimRight(options.PublicURL, "/")
	u, err := url.Parse(options.PublicURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.RawQuery != "" || u.Fragment != "" || u.User != nil || u.Path != "" {
		return nil, errors.New("public URL must be an absolute HTTP(S) origin")
	}
	a := &App{DB: db, Docs: &service.Documents{Store: db, BaseURL: options.PublicURL}, Options: options}
	a.registerGitHub(e)
	a.registerDevice(e)
	e.GET("/v1/auth/me", a.with("read", func(c echo.Context, p store.Credential) error {
		return c.JSON(200, map[string]any{"account": p.User, "credential_id": p.ID, "kind": p.Kind, "csrf_token": p.CSRF})
	}))
	e.POST("/v1/auth/logout", a.with("read", func(c echo.Context, p store.Credential) error {
		if _, err := db.RevokeCredential(p.User.ID, p.ID); err != nil {
			return a.fail(c, err)
		}
		c.SetCookie(&http.Cookie{Name: "fastmd_session", Value: "", Path: "/", HttpOnly: true, Secure: u.Scheme == "https", MaxAge: -1, SameSite: http.SameSiteLaxMode})
		return c.JSON(200, map[string]bool{"ok": true})
	}))
	e.GET("/v1/account/token", a.withSession(a.tokenMetadata))
	e.POST("/v1/account/token/reveal", a.withSession(a.revealToken))
	e.GET("/v1/connections", a.withSession(func(c echo.Context, p store.Credential) error {
		items, err := db.OAuthGrants(p.User.ID)
		if err != nil {
			return a.fail(c, err)
		}
		return c.JSON(200, map[string]any{"connections": items})
	}))
	e.DELETE("/v1/connections/:id", a.withSession(func(c echo.Context, p store.Credential) error {
		ok, err := db.RevokeGrant(p.User.ID, c.Param("id"))
		if err != nil {
			return a.fail(c, err)
		}
		if !ok {
			return a.fail(c, service.Failure("NOT_FOUND", "connection not found", 404))
		}
		return c.JSON(200, map[string]bool{"ok": true})
	}))
	e.POST("/v1/account/claim", a.with("account", func(c echo.Context, p store.Credential) error {
		var input struct {
			Token string `json:"token"`
		}
		if err := c.Bind(&input); err != nil || input.Token == "" {
			return a.fail(c, service.Failure("INVALID_REQUEST", "legacy token is required", 400))
		}
		count, err := db.ClaimLegacy(p.User.ID, input.Token)
		if err != nil {
			return a.fail(c, err)
		}
		return c.JSON(200, map[string]any{"claimed": count})
	}))
	e.GET("/v1/docs", a.with("read", func(c echo.Context, p store.Credential) error {
		offset, limit, err := pagination(c.QueryParam("cursor"), c.QueryParam("limit"))
		if err != nil {
			return a.fail(c, err)
		}
		result, err := a.Docs.List(p, offset, limit)
		if err != nil {
			return a.fail(c, err)
		}
		return c.JSON(200, result)
	}))
	e.GET("/v1/usage", a.with("read", func(c echo.Context, p store.Credential) error {
		result, err := a.Docs.Usage(p)
		if err != nil {
			return a.fail(c, err)
		}
		return c.JSON(200, result)
	}))
	e.POST("/v1/push", a.with("write", func(c echo.Context, p store.Credential) error {
		var input struct {
			Content   string `json:"content"`
			RequestID string `json:"request_id"`
		}
		if err := c.Bind(&input); err != nil {
			return a.fail(c, service.Failure("INVALID_REQUEST", "invalid JSON request", 400))
		}
		if input.RequestID == "" {
			input.RequestID = c.Request().Header.Get("Idempotency-Key")
		}
		result, err := a.Docs.Create(p, input.Content, input.RequestID)
		if err != nil {
			return a.fail(c, err)
		}
		return c.JSON(200, result)
	}))
	e.PUT("/v1/:id", a.with("write", func(c echo.Context, p store.Credential) error {
		var input struct {
			Content string `json:"content"`
		}
		if err := c.Bind(&input); err != nil {
			return a.fail(c, service.Failure("INVALID_REQUEST", "invalid JSON request", 400))
		}
		result, err := a.Docs.Update(p, c.Param("id"), input.Content)
		if err != nil {
			return a.fail(c, err)
		}
		return c.JSON(200, result)
	}))
	e.DELETE("/v1/:id", a.with("delete", func(c echo.Context, p store.Credential) error {
		if err := a.Docs.Delete(p, c.Param("id")); err != nil {
			return a.fail(c, err)
		}
		return c.JSON(200, map[string]any{"ok": true, "id": c.Param("id")})
	}))
	a.registerOAuth(e)
	a.registerMCP(e)
	return a, nil
}

func (a *App) fail(c echo.Context, err error) error {
	message := service.Describe(err)
	if errors.Is(err, store.ErrUnauthorized) {
		message = &service.Error{Code: "AUTH_REQUIRED", Message: "credential is invalid, expired or revoked", Status: 401}
	}
	return c.JSON(message.Status, message)
}

func (a *App) principal(c echo.Context) (store.Credential, error) {
	if header := c.Request().Header.Get("Authorization"); header != "" {
		raw, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || strings.TrimSpace(raw) == "" {
			return store.Credential{}, store.ErrUnauthorized
		}
		p, err := a.DB.Authenticate(raw)
		if err == nil && p.Kind == "session" {
			return p, store.ErrUnauthorized
		}
		return p, err
	}
	cookie, err := c.Cookie("fastmd_session")
	if err != nil {
		return store.Credential{}, store.ErrUnauthorized
	}
	p, err := a.DB.Authenticate(cookie.Value)
	if err != nil {
		return p, err
	}
	if p.Kind != "session" {
		return p, store.ErrUnauthorized
	}
	if c.Request().Method != "GET" && c.Request().Method != "HEAD" && c.Request().Method != "OPTIONS" {
		if origin := c.Request().Header.Get("Origin"); origin != "" && origin != a.Options.PublicURL {
			return p, service.Failure("CSRF_REJECTED", "request origin is not allowed", 403)
		}
		actual := c.Request().Header.Get("X-CSRF-Token")
		if actual == "" || subtle.ConstantTimeCompare([]byte(actual), []byte(p.CSRF)) != 1 {
			return p, service.Failure("CSRF_REJECTED", "CSRF token is required", 403)
		}
	}
	return p, nil
}

func (a *App) with(scope string, handler func(echo.Context, store.Credential) error) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Response().Header().Set("Cache-Control", "no-store")
		p, err := a.principal(c)
		if err != nil {
			return a.fail(c, err)
		}
		if err := service.Allowed(p, scope); err != nil {
			return a.fail(c, err)
		}
		return handler(c, p)
	}
}

func documentScopes(value string) (string, error) {
	if value == "" {
		return "read write delete", nil
	}
	seen := map[string]bool{}
	for _, scope := range strings.Fields(value) {
		if scope != "read" && scope != "write" && scope != "delete" {
			return "", service.Failure("INVALID_SCOPE", "supported scopes: read write delete", 400)
		}
		seen[scope] = true
	}
	if len(seen) == 0 {
		return "", service.Failure("INVALID_SCOPE", "at least one document scope is required", 400)
	}
	ordered := []string{}
	for _, scope := range []string{"read", "write", "delete"} {
		if seen[scope] {
			ordered = append(ordered, scope)
		}
	}
	return strings.Join(ordered, " "), nil
}

func pagination(cursor, limit string) (int, int, error) {
	offset := 0
	size := 50
	var err error
	if cursor != "" {
		offset, err = strconv.Atoi(cursor)
		if err != nil || offset < 0 {
			return 0, 0, service.Failure("INVALID_PAGE", "invalid cursor", 400)
		}
	}
	if limit != "" {
		size, err = strconv.Atoi(limit)
		if err != nil || size < 1 || size > 100 {
			return 0, 0, service.Failure("INVALID_PAGE", "limit must be 1-100", 400)
		}
	}
	return offset, size, nil
}

func safeReturn(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") || strings.ContainsAny(value, "\\\r\n") || parsed.IsAbs() || parsed.Host != "" {
		return "/dashboard"
	}
	return value
}
func (a *App) LoginPage(c echo.Context, returnPath string) error {
	return renderAuth(c, http.StatusOK, "login", authPageData{
		Title: "Sign in", Start: "/auth/github/start?next=" + url.QueryEscape(safeReturn(returnPath)),
	})
}
func (a *App) Session(c echo.Context) (store.Credential, error) {
	p, err := a.principal(c)
	if err == nil && p.Kind != "session" {
		return p, store.ErrUnauthorized
	}
	return p, err
}
func (a *App) withSession(handler func(echo.Context, store.Credential) error) echo.HandlerFunc {
	return a.with("account", func(c echo.Context, p store.Credential) error {
		if p.Kind != "session" {
			return a.fail(c, service.Failure("FORBIDDEN", "use the signed-in account page", 403))
		}
		return handler(c, p)
	})
}
func (a *App) tokenMetadata(c echo.Context, p store.Credential) error {
	return c.JSON(200, map[string]string{"masked_token": "fmd_live_••••••••••••••••••••••••"})
}
func (a *App) revealToken(c echo.Context, p store.Credential) error {
	raw, err := a.DB.DefaultToken(p.User.ID)
	if err != nil {
		return a.fail(c, err)
	}
	return c.JSON(200, map[string]string{"token": raw})
}
