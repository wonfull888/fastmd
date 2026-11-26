package app

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/wonfull888/fastmd/internal/store"
)

func TestMCPReverseProxyHost(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	e := echo.New()
	_, err = Register(e, db, Options{PublicURL: "https://fastmd.dev"})
	if err != nil {
		t.Fatal(err)
	}
	user, err := db.GitHubLogin(42, "proxy-test")
	if err != nil {
		t.Fatal(err)
	}
	token, err := db.DefaultToken(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(e)
	defer server.Close()
	loopback, _ := url.Parse(server.URL)
	for _, tc := range []struct {
		name, host, origin string
		status             int
	}{
		{"production proxy", "fastmd.dev", "", 200},
		{"case insensitive domain", "FASTMD.DEV", "", 200},
		{"explicit HTTPS port", "fastmd.dev:443", "", 200},
		{"loopback", loopback.Host, "", 200},
		{"localhost", "localhost:8080", "", 200},
		{"IPv6 loopback", "[::1]:8080", "", 200},
		{"untrusted domain", "attacker.test", "", 403},
		{"untrusted subdomain", "attacker.fastmd.dev", "", 403},
		{"cross origin", "fastmd.dev", "https://attacker.test", 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"proxy-regression","version":"1"}}}`
			req, err := http.NewRequest("POST", server.URL+"/mcp", strings.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			req.Host = tc.host
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json, text/event-stream")
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			response, err := server.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			data, _ := io.ReadAll(response.Body)
			if response.StatusCode != tc.status {
				t.Fatalf("status %d, want %d: %s", response.StatusCode, tc.status, data)
			}
			if tc.status == 200 && !strings.Contains(string(data), `"serverInfo"`) {
				t.Fatalf("MCP initialize did not complete: %s", data)
			}
		})
	}
}
