package app

import (
	"bytes"
	"html/template"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	fastmd "github.com/wonfull888/fastmd"
)

var authPages = template.Must(template.ParseFS(fastmd.WebFS, "web/templates/auth.html"))

type authField struct{ Name, Value string }
type authPermission struct{ Scope, Title, Description string }
type authPageData struct {
	Title, Start, Login, Code, CSRF, Name string
	Message, Error, Retry                 string
	Allowed                               bool
	Fields                                []authField
	Permissions                           []authPermission
}

func renderAuth(c echo.Context, status int, page string, data authPageData) error {
	var body bytes.Buffer
	if err := authPages.ExecuteTemplate(&body, page, data); err != nil {
		return err
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	// no-referrer makes browsers send Origin: null for native form POSTs,
	// which would be rejected by the existing CSRF origin validation.
	c.Response().Header().Set("Referrer-Policy", "same-origin")
	return c.HTML(status, body.String())
}

// Browser authorization endpoints share an error card. Protocol clients still
// receive the same JSON errors from endpoints used by CLI and MCP clients.
func authorizationError(c echo.Context, status int, code, message string) error {
	if strings.Contains(c.Request().Header.Get("Accept"), "text/html") {
		switch code {
		case "invalid_client", "invalid_request":
			message = "This connection request is incomplete or no longer valid. Start again from your app."
		case "invalid_target":
			message = "This request is for a different service. Start again from your app."
		case "invalid_scope":
			message = "This app requested permissions that fastmd does not support."
		case "login_required":
			message = "Your session expired. Sign in to fastmd, then start the connection again from your app."
		case "access_denied":
			message = "This authorization could not be verified. Start again from your app."
		}
		return renderAuth(c, status, "error", authPageData{
			Title: "Unable to connect", Message: message,
		})
	}
	return oauthError(c, status, code, message)
}

func consentPermissions(scopes string) []authPermission {
	var permissions []authPermission
	for _, scope := range strings.Fields(scopes) {
		switch scope {
		case "read":
			permissions = append(permissions, authPermission{scope, "Read documents and account usage", "Retrieve Markdown and list your documents."})
		case "write":
			permissions = append(permissions, authPermission{scope, "Create and update documents", "Publish new documents and edit existing ones."})
		case "delete":
			permissions = append(permissions, authPermission{scope, "Delete your documents", "Remove documents from your account."})
		}
	}
	return permissions
}

func renderDeviceResult(c echo.Context, allowed bool) error {
	title := "Connection cancelled"
	if allowed {
		title = "Terminal connected"
	}
	return renderAuth(c, http.StatusOK, "complete", authPageData{Title: title, Allowed: allowed})
}
