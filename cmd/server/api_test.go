package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wonfull888/fastmd/internal/store"
)

func TestDocumentAPIRegression(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	e, err := newServer(db)
	if err != nil {
		t.Fatal(err)
	}
	e.Logger.SetOutput(io.Discard)
	request := func(method, path, body, auth string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}
	for _, path := range []string{"/", "/dashboard", "/static/app.js", "/static/style.css", "/install.sh", "/v1/version"} {
		if rec := request("GET", path, "", ""); rec.Code != 200 {
			t.Fatalf("GET %s: %d %s", path, rec.Code, rec.Body)
		}
	}
	if rec := request("GET", "/v1/docs", "", ""); rec.Code != 401 {
		t.Fatalf("unauthenticated list: %d", rec.Code)
	}
	user, err := db.GitHubLogin(1, "owner")
	if err != nil {
		t.Fatal(err)
	}
	owner, err := db.DefaultToken(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	otherUser, err := db.GitHubLogin(2, "other")
	if err != nil {
		t.Fatal(err)
	}
	other, err := db.DefaultToken(otherUser.ID)
	if err != nil {
		t.Fatal(err)
	}
	created := request("POST", "/v1/push", `{"content":"# Original\n\nOriginal text"}`, "Bearer "+owner)
	var result struct{ ID, URL string }
	if created.Code != 200 || json.Unmarshal(created.Body.Bytes(), &result) != nil || len(result.ID) != 8 {
		t.Fatalf("create failed: %d %s", created.Code, created.Body)
	}
	before, _ := db.GetByID(result.ID)
	for _, tc := range []struct {
		name, id, body, auth string
		status               int
	}{
		{"no token", result.ID, `{"content":"bad"}`, "", 401},
		{"invalid auth scheme", result.ID, `{"content":"bad"}`, "owner", 401},
		{"empty bearer", result.ID, `{"content":"bad"}`, "Bearer   ", 401},
		{"wrong token", result.ID, `{"content":"bad"}`, "Bearer " + other, 404},
		{"missing document", "notFound", `{"content":"bad"}`, "Bearer " + owner, 404},
		{"empty content", result.ID, `{"content":" \n\t"}`, "Bearer " + owner, 400},
		{"missing content", result.ID, `{}`, "Bearer " + owner, 400},
		{"null", result.ID, `null`, "Bearer " + owner, 400},
		{"invalid JSON", result.ID, `{`, "Bearer " + owner, 400},
		{"wrong type", result.ID, `{"content":42}`, "Bearer " + owner, 400},
		{"body limit", result.ID, `{"content":"` + strings.Repeat("x", 1024*1024) + `"}`, "Bearer " + owner, 413},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := request("PUT", "/v1/"+tc.id, tc.body, tc.auth)
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.status, rec.Body)
			}
			doc, _ := db.GetByID(result.ID)
			if doc.Content != before.Content {
				t.Fatal("rejected update modified the document")
			}
		})
	}
	content := "# New title\n\nUpdated 中文 description.\n\n```sh\necho \"hello\"\n```\n"
	payload, _ := json.Marshal(map[string]string{"content": content})
	for attempt := 0; attempt < 2; attempt++ {
		rec := request("PUT", "/v1/"+result.ID, string(payload), "Bearer "+owner)
		var updated struct{ ID, URL string }
		if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &updated) != nil || updated != result {
			t.Fatalf("update changed ID/URL or failed: %d %s", rec.Code, rec.Body)
		}
	}
	after, _ := db.GetByID(result.ID)
	if after.CreatedAt != before.CreatedAt || after.Content != content {
		t.Fatal("update changed creation time or damaged content")
	}
	for _, suffix := range []string{"", ".md"} {
		rec := request("GET", "/"+result.ID+suffix, "", "")
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), "New title") || !strings.Contains(rec.Body.String(), "Updated 中文 description.") || rec.Header().Get("Cache-Control") != "no-cache" {
			t.Fatalf("updated page/metadata missing: %d %s", rec.Code, rec.Body)
		}
		req := httptest.NewRequest("GET", "/"+result.ID+suffix, nil)
		req.Header.Set("Accept", "text/plain")
		raw := httptest.NewRecorder()
		e.ServeHTTP(raw, req)
		if raw.Code != 200 || raw.Body.String() != content || !strings.HasPrefix(raw.Header().Get("Content-Type"), "text/plain") {
			t.Fatalf("raw Markdown mismatch: %d %q", raw.Code, raw.Body.String())
		}
	}
	listed := request("GET", "/v1/docs", "", "Bearer "+owner)
	var list struct{ Documents []struct{ ID, Title string } }
	if listed.Code != 200 || json.Unmarshal(listed.Body.Bytes(), &list) != nil || len(list.Documents) != 1 || list.Documents[0].Title != "New title" {
		t.Fatalf("list regression: %d %s", listed.Code, listed.Body)
	}
	if rec := request("DELETE", "/v1/"+result.ID, "", "Bearer "+other); rec.Code != 404 {
		t.Fatalf("wrong-token delete: %d", rec.Code)
	}
	if rec := request("DELETE", "/v1/"+result.ID, "", "Bearer "+owner); rec.Code != 200 {
		t.Fatalf("delete: %d", rec.Code)
	}
	if rec := request("PUT", "/v1/"+result.ID, string(payload), "Bearer "+owner); rec.Code != 404 {
		t.Fatalf("deleted document was updated: %d", rec.Code)
	}
	if rec := request(http.MethodGet, "/"+result.ID, "", ""); rec.Code != 404 {
		t.Fatalf("deleted document read: %d", rec.Code)
	}
}

func TestRateLimitRegression(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	e, err := newServer(db)
	if err != nil {
		t.Fatal(err)
	}
	e.Logger.SetOutput(io.Discard)
	for i := 0; i < 61; i++ {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/version", nil))
		want := 200
		if i == 60 {
			want = 429
		}
		if rec.Code != want {
			t.Fatalf("request %d: %d, want %d", i+1, rec.Code, want)
		}
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest("GET", "/static/style.css", nil))
	if rec.Code != 200 {
		t.Fatalf("static files should remain accessible: %d", rec.Code)
	}
}
