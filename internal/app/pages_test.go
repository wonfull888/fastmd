package app

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/wonfull888/fastmd/internal/store"
	"golang.org/x/net/html"
)

func authUIFixture(t *testing.T) (*echo.Echo, *App, *http.Cookie, string) {
	t.Helper()
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	e := echo.New()
	a, err := Register(e, db, Options{PublicURL: "https://fastmd.test", GitHub: GitHubConfig{ClientID: "test", ClientSecret: "secret"}})
	if err != nil {
		t.Fatal(err)
	}
	e.GET("/login", func(c echo.Context) error { return a.LoginPage(c, c.QueryParam("next")) })
	user, err := db.GitHubLogin(42, "alice")
	if err != nil {
		t.Fatal(err)
	}
	raw, credential, err := db.IssueCredential(user.ID, "session", "UI test", "read write delete account", time.Now().Unix()+60)
	if err != nil {
		t.Fatal(err)
	}
	return e, a, &http.Cookie{Name: "fastmd_session", Value: raw}, credential.CSRF
}

func browserForm(e *echo.Echo, path string, values url.Values, cookie *http.Cookie, origin string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", path, strings.NewReader(values.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Accept", "text/html")
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	e.ServeHTTP(w, r)
	return w
}

func assertAuthPage(t *testing.T, response *httptest.ResponseRecorder, title string) {
	t.Helper()
	if !strings.HasPrefix(response.Header().Get("Content-Type"), "text/html") || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("auth page must be non-cacheable HTML")
	}
	if response.Header().Get("Referrer-Policy") != "same-origin" {
		t.Fatal("auth pages must preserve the origin of native form submissions")
	}
	for _, content := range []string{"<!doctype html>", `lang="en"`, `/static/auth.css`, `name="viewport"`, `content="noindex, nofollow"`, `id="auth-title">` + title, "Published documents are accessible by link."} {
		if !strings.Contains(response.Body.String(), content) {
			t.Fatalf("auth page missing %q", content)
		}
	}
}

// Read the actual form as a browser would, rather than rebuilding its hidden
// inputs in the test. This protects the OAuth round trip when templates change.
func hiddenForm(t *testing.T, body string) url.Values {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	values := url.Values{}
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "input" {
			attrs := map[string]string{}
			for _, attr := range node.Attr {
				attrs[attr.Key] = attr.Val
			}
			if attrs["type"] == "hidden" {
				values.Add(attrs["name"], attrs["value"])
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return values
}

func TestAuthLoginReturnAndErrors(t *testing.T) {
	e, _, _, _ := authUIFixture(t)
	page := serve(e, "GET", "/login?next="+url.QueryEscape("/auth/device?user_code=ABCD-EFGH"), "", nil, "", "")
	assertAuthPage(t, page, "Sign in to fastmd")
	if !strings.Contains(page.Body.String(), "/auth/github/start?next=%2Fauth%2Fdevice%3Fuser_code%3DABCD-EFGH") || !strings.Contains(page.Body.String(), "Public profile only. No repository access.") {
		t.Fatal("login lost its return path or privacy explanation")
	}
	page = serve(e, "GET", "/login?next="+url.QueryEscape("//evil.test"), "", nil, "", "")
	if !strings.Contains(page.Body.String(), "/auth/github/start?next=%2Fdashboard") {
		t.Fatal("unsafe return path accepted")
	}
	r := httptest.NewRequest("GET", "/auth/github/callback?state=invalid&code=test", nil)
	r.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()
	e.ServeHTTP(w, r)
	assertAuthPage(t, w, "Unable to sign in")
	if w.Code != 400 || !strings.Contains(w.Body.String(), "Try again") {
		t.Fatal("expired GitHub sign-in should show a retry action")
	}
	r = httptest.NewRequest("GET", "/oauth/authorize?client_id=unknown", nil)
	r.Header.Set("Accept", "text/html")
	w = httptest.NewRecorder()
	e.ServeHTTP(w, r)
	assertAuthPage(t, w, "Unable to connect")
	if w.Code != 400 || !strings.Contains(w.Body.String(), "Start again from your app.") {
		t.Fatal("invalid OAuth request should show recovery instructions")
	}
	protocolError := serve(e, "GET", "/oauth/authorize?client_id=unknown", "", nil, "", "")
	if protocolError.Code != 400 || !strings.Contains(protocolError.Body.String(), `"error":"invalid_client"`) {
		t.Fatal("OAuth protocol error must remain JSON")
	}
}

func TestConsentRenderedFormAndPermissions(t *testing.T) {
	e, a, cookie, csrf := authUIFixture(t)
	client, err := a.DB.RegisterClient(`Codex <script>alert(1)</script>`, []string{"http://127.0.0.1:5173/callback?existing=1"})
	if err != nil {
		t.Fatal(err)
	}
	for _, scope := range []string{"read", "write", "delete", "read write delete"} {
		t.Run(scope, func(t *testing.T) {
			values := url.Values{"client_id": {client.ID}, "redirect_uri": {client.RedirectURIs[0]}, "response_type": {"code"}, "state": {"random&state=123"}, "code_challenge": {strings.Repeat("a", 43)}, "code_challenge_method": {"S256"}, "resource": {"https://fastmd.test/mcp"}, "scope": {scope}}
			page := serve(e, "GET", "/oauth/authorize?"+values.Encode(), "", cookie, "", "")
			assertAuthPage(t, page, "Connect Codex &lt;script&gt;alert(1)&lt;/script&gt;")
			if page.Code != 200 || strings.Contains(page.Body.String(), "<script>") {
				t.Fatal("client name must be displayed safely")
			}
			for requested, title := range map[string]string{"read": "Read documents and account usage", "write": "Create and update documents", "delete": "Delete your documents"} {
				if strings.Contains(page.Body.String(), title) != strings.Contains(scope, requested) {
					t.Fatalf("displayed permissions do not match requested %s scope", scope)
				}
			}
			form := hiddenForm(t, page.Body.String())
			for key := range values {
				if form.Get(key) != values.Get(key) {
					t.Fatalf("hidden field %s did not survive rendering", key)
				}
			}
			if form.Get("csrf_token") != csrf {
				t.Fatal("consent CSRF token missing")
			}
			form.Set("decision", "allow")
			badCSRF := form.Get("csrf_token")
			form.Set("csrf_token", "wrong")
			if bad := browserForm(e, "/oauth/authorize", form, cookie, "https://fastmd.test"); bad.Code != 403 {
				t.Fatal("consent accepted invalid CSRF")
			}
			form.Set("csrf_token", badCSRF)
			if bad := browserForm(e, "/oauth/authorize", form, cookie, "https://evil.test"); bad.Code != 403 {
				t.Fatal("consent accepted a cross-origin submission")
			}
			for _, decision := range []string{"allow", "deny"} {
				form.Set("decision", decision)
				done := browserForm(e, "/oauth/authorize", form, cookie, "https://fastmd.test")
				target, err := url.Parse(done.Header().Get("Location"))
				if err != nil || done.Code != 303 || target.Query().Get("state") != values.Get("state") || target.Query().Get("existing") != "1" {
					t.Fatal("rendered consent form lost the callback parameters")
				}
				if decision == "allow" && target.Query().Get("code") == "" {
					t.Fatal("authorization code missing")
				}
				if decision == "deny" && (target.Query().Get("error") != "access_denied" || target.Query().Get("code") != "") {
					t.Fatal("cancellation granted access")
				}
			}
		})
	}
}

func TestDeviceRenderedFormResultsAndRejection(t *testing.T) {
	e, a, cookie, _ := authUIFixture(t)
	for _, decision := range []string{"allow", "deny"} {
		t.Run(decision, func(t *testing.T) {
			code, userCode, err := a.DB.CreateDevice()
			if err != nil {
				t.Fatal(err)
			}
			page := serve(e, "GET", "/auth/device?user_code="+userCode, "", cookie, "", "")
			assertAuthPage(t, page, "Connect this terminal")
			if !strings.Contains(page.Body.String(), `value="`+userCode+`"`) || !strings.Contains(page.Body.String(), "alice") {
				t.Fatal("confirmation code or signed-in account missing")
			}
			form := hiddenForm(t, page.Body.String())
			form.Set("user_code", userCode)
			form.Set("decision", decision)
			if bad := browserForm(e, "/auth/device", form, cookie, "https://evil.test"); bad.Code != 403 {
				t.Fatal("device form accepted cross-origin submission")
			}
			validCSRF := form.Get("csrf_token")
			form.Set("csrf_token", "wrong")
			if bad := browserForm(e, "/auth/device", form, cookie, "https://fastmd.test"); bad.Code != 403 {
				t.Fatal("device form accepted invalid CSRF")
			}
			form.Set("csrf_token", validCSRF)
			done := browserForm(e, "/auth/device", form, cookie, "https://fastmd.test")
			title := "Connection cancelled"
			if decision == "allow" {
				title = "Terminal connected"
			}
			assertAuthPage(t, done, title)
			if done.Code != 200 || !strings.Contains(done.Body.String(), `href="/dashboard"`) {
				t.Fatal("completion did not offer a return to fastmd")
			}
			token, _, pollError := a.DB.PollDevice(code)
			if decision == "deny" && (!errors.Is(pollError, store.ErrUnauthorized) || token != "") {
				t.Fatal("cancelled terminal received access")
			}
			if decision == "allow" && (pollError != nil || token == "") {
				t.Fatal("approved terminal did not receive access")
			}
			used := browserForm(e, "/auth/device", form, cookie, "https://fastmd.test")
			assertAuthPage(t, used, "Connect this terminal")
			if used.Code != 400 || !strings.Contains(used.Body.String(), `role="alert"`) {
				t.Fatal("used code should show an inline error, not success")
			}
		})
	}
}
