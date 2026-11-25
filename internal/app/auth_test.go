package app

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/wonfull888/fastmd/internal/store"
)

func serve(e *echo.Echo, method, path, body string, cookie *http.Cookie, csrf, bearer string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	if csrf != "" {
		r.Header.Set("X-CSRF-Token", csrf)
	}
	if bearer != "" {
		r.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	e.ServeHTTP(w, r)
	return w
}
func TestGitHubLoginAndFixedToken(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	challenge := ""
	calls := 0
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/token":
			calls++
			r.ParseForm()
			digest := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
			if r.Form.Get("client_secret") != "test-secret" || base64.RawURLEncoding.EncodeToString(digest[:]) != challenge {
				http.Error(w, "invalid exchange", 400)
				return
			}
			fmt.Fprint(w, `{"access_token":"github-test-access","expires_in":28800,"refresh_token":"github-test-refresh"}`)
		case "/user":
			if r.Header.Get("Authorization") != "Bearer github-test-access" {
				http.Error(w, "invalid token", 401)
				return
			}
			fmt.Fprint(w, `{"id":42,"login":"alice"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer provider.Close()
	e := echo.New()
	a, err := Register(e, db, Options{PublicURL: "https://fastmd.test", GitHub: GitHubConfig{ClientID: "test-id", ClientSecret: "test-secret", AuthorizeURL: provider.URL + "/authorize", TokenURL: provider.URL + "/token", UserURL: provider.URL + "/user"}})
	if err != nil {
		t.Fatal(err)
	}
	signIn := func(next string) *http.Cookie {
		t.Helper()
		start := serve(e, "GET", "/auth/github/start?next="+url.QueryEscape(next), "", nil, "", "")
		target, err := url.Parse(start.Header().Get("Location"))
		if err != nil || start.Code != 302 {
			t.Fatalf("start: %d", start.Code)
		}
		if target.Query().Get("scope") != "" || target.Query().Get("client_secret") != "" {
			t.Fatal("unexpected permissions or secret")
		}
		challenge = target.Query().Get("code_challenge")
		stateCookie := start.Result().Cookies()[0]
		if !stateCookie.HttpOnly || !stateCookie.Secure {
			t.Fatal("state cookie is not protected")
		}
		callback := "/auth/github/callback?state=" + target.Query().Get("state") + "&code=test-code"
		if bad := serve(e, "GET", callback, "", nil, "", ""); bad.Code != 400 {
			t.Fatal("missing state cookie accepted")
		}
		done := serve(e, "GET", callback, "", stateCookie, "", "")
		if done.Code != 303 || done.Header().Get("Location") != "/dashboard" {
			t.Fatalf("callback: %d %s", done.Code, done.Body)
		}
		if replay := serve(e, "GET", callback, "", stateCookie, "", ""); replay.Code != 400 {
			t.Fatal("callback replay accepted")
		}
		for _, cookie := range done.Result().Cookies() {
			if cookie.Name == "fastmd_session" {
				if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode {
					t.Fatal("session is not protected")
				}
				return cookie
			}
		}
		t.Fatal("session missing")
		return nil
	}
	cookie := signIn("//evil.example")
	me := serve(e, "GET", "/v1/auth/me", "", cookie, "", "")
	var identity struct {
		Account store.User `json:"account"`
		CSRF    string     `json:"csrf_token"`
	}
	if err := json.Unmarshal(me.Body.Bytes(), &identity); err != nil || identity.Account.GitHubID != 42 {
		t.Fatalf("me: %s", me.Body)
	}
	api, err := db.DefaultToken(identity.Account.ID)
	if err != nil {
		t.Fatal(err)
	}
	metadata := serve(e, "GET", "/v1/account/token", "", cookie, "", "")
	if metadata.Code != 200 || strings.Contains(metadata.Body.String(), api) {
		t.Fatal("default response exposed token")
	}
	for _, csrf := range []string{"", "wrong"} {
		if response := serve(e, "POST", "/v1/account/token/reveal", "{}", cookie, csrf, ""); response.Code != 403 {
			t.Fatal("reveal missing CSRF protection")
		}
	}
	revealed := serve(e, "POST", "/v1/account/token/reveal", "{}", cookie, identity.CSRF, "")
	if revealed.Code != 200 || !strings.Contains(revealed.Body.String(), api) || revealed.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("reveal failed")
	}
	if response := serve(e, "POST", "/v1/account/token/reveal", "{}", nil, "", api); response.Code != 403 {
		t.Fatal("API credential exposed account secret")
	}
	for _, path := range []string{"/v1/keys", "/v1/keys/id/rotate", "/v1/auth/request", "/v1/auth/verify"} {
		if response := serve(e, "POST", path, "{}", cookie, identity.CSRF, ""); response.Code != 404 && response.Code != 405 {
			t.Fatalf("removed endpoint %s: %d", path, response.Code)
		}
	}
	otherCookie := signIn("/dashboard")
	other, err := a.Session(e.NewContext(func() *http.Request { r := httptest.NewRequest("GET", "/", nil); r.AddCookie(otherCookie); return r }(), httptest.NewRecorder()))
	if err != nil {
		t.Fatal(err)
	}
	if same, _ := db.DefaultToken(other.User.ID); same != api {
		t.Fatal("login replaced API token")
	}
	if response := serve(e, "POST", "/v1/auth/logout", "{}", cookie, identity.CSRF, ""); response.Code != 200 {
		t.Fatal("logout failed")
	}
	if response := serve(e, "GET", "/v1/auth/me", "", cookie, "", ""); response.Code != 401 {
		t.Fatal("session survived logout")
	}
	if _, err := db.Authenticate(api); err != nil {
		t.Fatal("logout invalidated API token")
	}
	if calls != 2 {
		t.Fatalf("unexpected provider calls %d", calls)
	}
}

func TestDeviceHTTPAuthorization(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	e := echo.New()
	_, err = Register(e, db, Options{PublicURL: "https://fastmd.test", GitHub: GitHubConfig{ClientID: "test", ClientSecret: "secret"}})
	if err != nil {
		t.Fatal(err)
	}
	user, err := db.GitHubLogin(42, "alice")
	if err != nil {
		t.Fatal(err)
	}
	raw, key, err := db.IssueCredential(user.ID, "session", "test", "read write delete account", time.Now().Unix()+60)
	if err != nil {
		t.Fatal(err)
	}
	cookie := &http.Cookie{Name: "fastmd_session", Value: raw}
	response := serve(e, "POST", "/v1/auth/device", "{}", nil, "", "")
	var device struct {
		Code string `json:"device_code"`
		User string `json:"user_code"`
	}
	json.Unmarshal(response.Body.Bytes(), &device)
	if response.Code != 200 || device.Code == "" {
		t.Fatal("device request failed")
	}
	if page := serve(e, "GET", "/auth/device?user_code="+device.User, "", nil, "", ""); !strings.Contains(page.Body.String(), "Continue with GitHub") {
		t.Fatal("device did not require login")
	}
	body := url.Values{"user_code": {device.User}, "decision": {"allow"}, "csrf_token": {key.CSRF}}.Encode()
	req := httptest.NewRequest("POST", "/auth/device", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("device approval: %d %s", rec.Code, rec.Body)
	}
	response = serve(e, "POST", "/v1/auth/device/token", `{"device_code":"`+device.Code+`"}`, nil, "", "")
	var result struct {
		Token string `json:"token"`
	}
	json.Unmarshal(response.Body.Bytes(), &result)
	if response.Code != 200 || result.Token == "" {
		t.Fatalf("poll: %d %s", response.Code, response.Body)
	}
	if p, err := db.Authenticate(result.Token); err != nil || p.Kind != "cli" {
		t.Fatal("wrong device credential")
	}
}
