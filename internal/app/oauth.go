package app

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/wonfull888/fastmd/internal/store"
)

func (a *App) registerOAuth(e *echo.Echo) {
	resource := func(c echo.Context) error {
		return c.JSON(200, map[string]any{"resource": a.Options.PublicURL + "/mcp", "authorization_servers": []string{a.Options.PublicURL}, "scopes_supported": []string{"read", "write", "delete"}, "bearer_methods_supported": []string{"header"}})
	}
	e.GET("/.well-known/oauth-protected-resource", resource)
	e.GET("/.well-known/oauth-protected-resource/mcp", resource)
	e.GET("/.well-known/oauth-authorization-server", func(c echo.Context) error {
		return c.JSON(200, map[string]any{
			"issuer": a.Options.PublicURL, "authorization_endpoint": a.Options.PublicURL + "/oauth/authorize", "token_endpoint": a.Options.PublicURL + "/oauth/token", "registration_endpoint": a.Options.PublicURL + "/oauth/register",
			"response_types_supported": []string{"code"}, "grant_types_supported": []string{"authorization_code", "refresh_token"}, "token_endpoint_auth_methods_supported": []string{"none"}, "code_challenge_methods_supported": []string{"S256"}, "scopes_supported": []string{"read", "write", "delete"}, "client_id_metadata_document_supported": false,
		})
	})
	e.POST("/oauth/register", a.registerClient)
	e.GET("/oauth/authorize", a.authorize)
	e.POST("/oauth/authorize", a.authorize)
	e.POST("/oauth/token", a.token)
}

func oauthError(c echo.Context, status int, code, description string) error {
	return c.JSON(status, map[string]string{"error": code, "error_description": description})
}
func validRedirect(value string) bool {
	u, err := url.Parse(value)
	if err != nil || u.Host == "" || u.User != nil || u.Fragment != "" {
		return false
	}
	if u.Scheme == "https" {
		return true
	}
	ip := net.ParseIP(u.Hostname())
	return u.Scheme == "http" && (u.Hostname() == "localhost" || (ip != nil && ip.IsLoopback()))
}
func (a *App) registerClient(c echo.Context) error {
	var input struct {
		ClientName    string   `json:"client_name"`
		RedirectURIs  []string `json:"redirect_uris"`
		AuthMethod    string   `json:"token_endpoint_auth_method"`
		GrantTypes    []string `json:"grant_types"`
		ResponseTypes []string `json:"response_types"`
	}
	if err := c.Bind(&input); err != nil || len(input.RedirectURIs) == 0 || len(input.RedirectURIs) > 10 {
		return oauthError(c, 400, "invalid_client_metadata", "redirect_uris are required")
	}
	for _, uri := range input.RedirectURIs {
		if len(uri) > 2048 || !validRedirect(uri) {
			return oauthError(c, 400, "invalid_redirect_uri", "use HTTPS or a loopback HTTP redirect URI")
		}
	}
	if input.AuthMethod != "" && input.AuthMethod != "none" {
		return oauthError(c, 400, "invalid_client_metadata", "only public PKCE clients are supported")
	}
	for _, grant := range input.GrantTypes {
		if grant != "authorization_code" && grant != "refresh_token" {
			return oauthError(c, 400, "invalid_client_metadata", "unsupported grant type")
		}
	}
	for _, response := range input.ResponseTypes {
		if response != "code" {
			return oauthError(c, 400, "invalid_client_metadata", "only code responses are supported")
		}
	}
	input.ClientName = strings.TrimSpace(input.ClientName)
	if input.ClientName == "" {
		input.ClientName = "MCP client"
	}
	if len(input.ClientName) > 100 {
		return oauthError(c, 400, "invalid_client_metadata", "client name is too long")
	}
	client, err := a.DB.RegisterClient(input.ClientName, input.RedirectURIs)
	if err != nil {
		return a.fail(c, err)
	}
	return c.JSON(201, map[string]any{"client_id": client.ID, "client_id_issued_at": time.Now().Unix(), "client_name": client.Name, "redirect_uris": client.RedirectURIs, "token_endpoint_auth_method": "none", "grant_types": []string{"authorization_code", "refresh_token"}, "response_types": []string{"code"}})
}

var challengeFormat = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
var verifierFormat = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)

func (a *App) authorize(c echo.Context) error {
	c.Response().Header().Set("Cache-Control", "no-store")
	if err := c.Request().ParseForm(); err != nil {
		return authorizationError(c, 400, "invalid_request", "invalid authorization request")
	}
	values := c.Request().Form
	if c.Request().Method == "POST" {
		values = c.Request().PostForm
	}
	client, err := a.DB.OAuthClient(values.Get("client_id"))
	if err != nil {
		return authorizationError(c, 400, "invalid_client", "unknown client")
	}
	redirect := values.Get("redirect_uri")
	if !slices.Contains(client.RedirectURIs, redirect) || !validRedirect(redirect) {
		return authorizationError(c, 400, "invalid_request", "redirect URI does not match the registration")
	}
	if values.Get("response_type") != "code" || values.Get("code_challenge_method") != "S256" || !challengeFormat.MatchString(values.Get("code_challenge")) {
		return authorizationError(c, 400, "invalid_request", "authorization code with S256 PKCE is required")
	}
	if values.Get("resource") != a.Options.PublicURL+"/mcp" {
		return authorizationError(c, 400, "invalid_target", "resource must identify this MCP server")
	}
	if len(values.Get("state")) < 8 || len(values.Get("state")) > 2048 {
		return authorizationError(c, 400, "invalid_request", "a valid state is required")
	}
	scopes, err := documentScopes(values.Get("scope"))
	if err != nil {
		return authorizationError(c, 400, "invalid_scope", "unsupported scope")
	}
	cookie, err := c.Cookie("fastmd_session")
	var principal store.Credential
	if err == nil {
		principal, err = a.DB.Authenticate(cookie.Value)
		if principal.Kind != "session" {
			err = store.ErrUnauthorized
		}
	}
	if err != nil {
		if c.Request().Method != "GET" {
			return authorizationError(c, 401, "login_required", "sign in before authorizing")
		}
		return a.LoginPage(c, "/oauth/authorize?"+values.Encode())
	}
	if c.Request().Method == "GET" {
		fields := []authField{}
		for _, name := range []string{"client_id", "redirect_uri", "response_type", "state", "code_challenge", "code_challenge_method", "resource"} {
			fields = append(fields, authField{name, values.Get(name)})
		}
		fields = append(fields, authField{"scope", scopes})
		return renderAuth(c, 200, "consent", authPageData{Title: "Connect " + client.Name, Name: client.Name, Login: principal.User.GitHubLogin, Fields: fields, CSRF: principal.CSRF, Permissions: consentPermissions(scopes)})
	}
	if origin := c.Request().Header.Get("Origin"); origin != "" && origin != a.Options.PublicURL {
		return authorizationError(c, 403, "access_denied", "invalid request origin")
	}
	csrf := values.Get("csrf_token")
	if csrf == "" || subtle.ConstantTimeCompare([]byte(csrf), []byte(principal.CSRF)) != 1 {
		return authorizationError(c, 403, "access_denied", "invalid CSRF token")
	}
	target, _ := url.Parse(redirect)
	query := target.Query()
	query.Set("state", values.Get("state"))
	query.Set("iss", a.Options.PublicURL)
	if values.Get("decision") != "allow" {
		query.Set("error", "access_denied")
	} else {
		code, err := a.DB.AuthorizationCode(principal.User.ID, client.ID, redirect, values.Get("code_challenge"), values.Get("resource"), scopes)
		if err != nil {
			return a.fail(c, err)
		}
		query.Set("code", code)
	}
	target.RawQuery = query.Encode()
	return c.Redirect(http.StatusSeeOther, target.String())
}

func (a *App) token(c echo.Context) error {
	c.Response().Header().Set("Cache-Control", "no-store")
	c.Response().Header().Set("Pragma", "no-cache")
	if err := c.Request().ParseForm(); err != nil {
		return oauthError(c, 400, "invalid_request", "invalid token request")
	}
	form := c.Request().PostForm
	clientID := form.Get("client_id")
	if _, err := a.DB.OAuthClient(clientID); err != nil {
		return oauthError(c, 400, "invalid_client", "unknown client")
	}
	if form.Get("resource") != a.Options.PublicURL+"/mcp" {
		return oauthError(c, 400, "invalid_target", "resource must identify this MCP server")
	}
	var result store.OAuthTokens
	var err error
	switch form.Get("grant_type") {
	case "authorization_code":
		verifier := form.Get("code_verifier")
		if !verifierFormat.MatchString(verifier) {
			return oauthError(c, 400, "invalid_grant", "a valid PKCE verifier is required")
		}
		digest := sha256.Sum256([]byte(verifier))
		challenge := base64.RawURLEncoding.EncodeToString(digest[:])
		result, err = a.DB.ExchangeCode(form.Get("code"), clientID, form.Get("redirect_uri"), challenge, form.Get("resource"))
	case "refresh_token":
		result, err = a.DB.RefreshOAuth(form.Get("refresh_token"), clientID)
	default:
		return oauthError(c, 400, "unsupported_grant_type", "use authorization_code or refresh_token")
	}
	if err != nil {
		return oauthError(c, 400, "invalid_grant", "authorization grant is invalid, expired or already used")
	}
	return c.JSON(200, result)
}
