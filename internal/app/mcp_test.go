package app

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wonfull888/fastmd/internal/store"
)

type bearerTransport struct{ Token string }

func (b bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.Token)
	return http.DefaultTransport.RoundTrip(r)
}

func TestMCPToolsAndOAuth(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	e := echo.New()
	server := httptest.NewServer(e)
	defer server.Close()
	a, err := Register(e, db, Options{PublicURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	user, err := db.GitHubLogin(42, "alice")
	if err != nil {
		t.Fatal(err)
	}
	token, _ := db.DefaultToken(user.ID)
	sessionRaw, sessionKey, err := db.IssueCredential(user.ID, "session", "test", "read write delete account", time.Now().Unix()+60)
	if err != nil {
		t.Fatal(err)
	}
	cookie := &http.Cookie{Name: "fastmd_session", Value: sessionRaw}
	unauthorized := serve(e, "POST", "/mcp", "{}", nil, "", "")
	if unauthorized.Code != 401 || !strings.Contains(unauthorized.Header().Get("WWW-Authenticate"), "resource_metadata") {
		t.Fatal("missing MCP auth discovery")
	}
	for _, path := range []string{"/.well-known/oauth-protected-resource/mcp", "/.well-known/oauth-authorization-server"} {
		if rec := serve(e, "GET", path, "", nil, "", ""); rec.Code != 200 {
			t.Fatal("missing discovery")
		}
	}
	registration := serve(e, "POST", "/oauth/register", `{"client_name":"test agent","redirect_uris":["http://127.0.0.1:5173/callback"]}`, nil, "", "")
	var client struct {
		ID string `json:"client_id"`
	}
	json.Unmarshal(registration.Body.Bytes(), &client)
	if registration.Code != 201 || client.ID == "" {
		t.Fatal("client registration failed")
	}
	if bad := serve(e, "POST", "/oauth/register", `{"redirect_uris":["http://evil.test/callback"]}`, nil, "", ""); bad.Code != 400 {
		t.Fatal("unsafe redirect accepted")
	}
	verifier := strings.Repeat("v", 43)
	digest := sha256.Sum256([]byte(verifier))
	values := url.Values{"client_id": {client.ID}, "redirect_uri": {"http://127.0.0.1:5173/callback"}, "response_type": {"code"}, "state": {"random-state-123"}, "code_challenge": {base64.RawURLEncoding.EncodeToString(digest[:])}, "code_challenge_method": {"S256"}, "resource": {server.URL + "/mcp"}, "scope": {"read write delete"}}
	if page := serve(e, "GET", "/oauth/authorize?"+values.Encode(), "", nil, "", ""); !strings.Contains(page.Body.String(), "Continue with GitHub") {
		t.Fatal("MCP auth did not require login")
	}
	if page := serve(e, "GET", "/oauth/authorize?"+values.Encode(), "", cookie, "", ""); !strings.Contains(page.Body.String(), "test agent") {
		t.Fatal("consent missing")
	}
	values.Set("decision", "allow")
	values.Set("csrf_token", sessionKey.CSRF)
	req := httptest.NewRequest("POST", "/oauth/authorize", strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	redirect, _ := url.Parse(rec.Header().Get("Location"))
	if rec.Code != 303 || redirect.Query().Get("code") == "" || redirect.Query().Get("state") != "random-state-123" {
		t.Fatalf("consent failed: %d %s", rec.Code, rec.Body)
	}
	form := url.Values{"grant_type": {"authorization_code"}, "client_id": {client.ID}, "redirect_uri": {values.Get("redirect_uri")}, "resource": {server.URL + "/mcp"}, "code": {redirect.Query().Get("code")}, "code_verifier": {verifier}}
	exchange := func(v url.Values) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(v.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		e.ServeHTTP(w, r)
		return w
	}
	form.Set("code_verifier", strings.Repeat("x", 43))
	if wrong := exchange(form); wrong.Code != 400 {
		t.Fatal("wrong PKCE accepted")
	}
	form.Set("code_verifier", verifier)
	result := exchange(form)
	var tokens store.OAuthTokens
	json.Unmarshal(result.Body.Bytes(), &tokens)
	if result.Code != 200 || tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatalf("exchange failed %s", result.Body)
	}
	if replay := exchange(form); replay.Code != 400 {
		t.Fatal("authorization code replay accepted")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	connect := func(raw string) *mcp.ClientSession {
		t.Helper()
		client := mcp.NewClient(&mcp.Implementation{Name: "fastmd-integration-test", Version: "1"}, nil)
		session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: server.URL + "/mcp", HTTPClient: &http.Client{Transport: bearerTransport{raw}}, DisableStandaloneSSE: true}, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { session.Close() })
		return session
	}
	sdk := connect(tokens.AccessToken)
	tools, err := sdk.ListTools(ctx, nil)
	if err != nil || len(tools.Tools) != 6 {
		t.Fatalf("six tools unavailable: %+v %v", tools, err)
	}
	call := func(name string, args any, wantError bool) map[string]any {
		t.Helper()
		result, err := sdk.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		if result.IsError != wantError {
			t.Fatalf("%s error=%v, want=%v: %+v", name, result.IsError, wantError, result.Content)
		}
		data, _ := json.Marshal(result.StructuredContent)
		var value map[string]any
		json.Unmarshal(data, &value)
		return value
	}
	created := call("create_document", map[string]string{"content": "# Original\n\n中文 😀\n", "request_id": "mcp-retry-1"}, false)
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatal("create result missing ID")
	}
	if again := call("create_document", map[string]string{"content": "# Original\n\n中文 😀\n", "request_id": "mcp-retry-1"}, false); again["id"] != id {
		t.Fatal("retry duplicated document")
	}
	call("create_document", map[string]string{"content": "different", "request_id": "mcp-retry-1"}, true)
	updated := call("update_document", map[string]string{"id": id, "content": "# Updated\r\n\n```sh\necho x\n```\n"}, false)
	if updated["url"] != created["url"] || updated["created_at"] != created["created_at"] {
		t.Fatal("update changed document identity")
	}
	call("update_document", map[string]string{"id": id, "content": " "}, true)
	got := call("get_document", map[string]string{"id": id}, false)
	if got["content"] != "# Updated\r\n\n```sh\necho x\n```\n" {
		t.Fatal("content changed")
	}
	call("list_documents", map[string]any{"limit": 1}, false)
	if usage := call("get_usage", map[string]any{}, false); usage["document_count"] != float64(1) {
		t.Fatal("usage incorrect")
	}
	other, _ := db.GitHubLogin(43, "bob")
	otherToken, _ := db.DefaultToken(other.ID)
	otherSession := connect(otherToken)
	denied, err := otherSession.CallTool(ctx, &mcp.CallToolParams{Name: "update_document", Arguments: map[string]string{"id": id, "content": "stolen"}})
	if err != nil || !denied.IsError {
		t.Fatal("cross-account modification allowed")
	}
	call("delete_document", map[string]string{"id": id}, false)
	call("create_document", map[string]string{"content": "# Original\n\n中文 😀\n", "request_id": "mcp-retry-1"}, true)
	call("get_document", map[string]string{"id": id}, true)
	// API-token access follows the exact same remote protocol.
	apiSDK := connect(token)
	if listed, err := apiSDK.ListTools(ctx, nil); err != nil || len(listed.Tools) != 6 {
		t.Fatal("API token MCP access failed")
	}
	badOrigin := httptest.NewRequest("POST", "/mcp", strings.NewReader("{}"))
	badOrigin.Header.Set("Origin", "https://evil.test")
	badOrigin.Header.Set("Authorization", "Bearer "+token)
	originRec := httptest.NewRecorder()
	e.ServeHTTP(originRec, badOrigin)
	if originRec.Code != 403 {
		t.Fatal("unexpected MCP origin accepted")
	}
	refresh := url.Values{"grant_type": {"refresh_token"}, "client_id": {client.ID}, "resource": {server.URL + "/mcp"}, "refresh_token": {tokens.RefreshToken}}
	refreshed := exchange(refresh)
	var next store.OAuthTokens
	json.Unmarshal(refreshed.Body.Bytes(), &next)
	if refreshed.Code != 200 || next.RefreshToken == tokens.RefreshToken {
		t.Fatal("refresh did not rotate")
	}
	if replay := exchange(refresh); replay.Code != 400 {
		t.Fatal("refresh replay accepted")
	}
	if _, err := db.Authenticate(next.AccessToken); err == nil {
		t.Fatal("compromised grant was not revoked")
	}
	if _, err := db.Authenticate(token); err != nil {
		t.Fatal("OAuth affected permanent API token")
	}
	_ = a
}
