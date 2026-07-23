package tests

import (
	"strings"
	"testing"

	"github.com/wonfull888/fastmd/internal/render"
	"github.com/wonfull888/fastmd/internal/store"
)

// ── Render package smoke test ───────────────────────────────────────

func TestRenderToHTML(t *testing.T) {
	markdown := "# Hello\n\nThis is **bold** text."
	html, err := render.ToHTML(markdown)
	if err != nil {
		t.Fatalf("ToHTML failed: %v", err)
	}
	if !strings.Contains(html, "<h1") || !strings.Contains(html, "Hello") {
		t.Errorf("expected <h1>Hello in output, got: %s", html)
	}
	if !strings.Contains(html, "<strong>bold</strong>") {
		t.Errorf("expected <strong>bold</strong> in output, got: %s", html)
	}
}

func TestRenderToHTML_CodeBlock(t *testing.T) {
	markdown := "```go\nfunc main() {}\n```"
	html, err := render.ToHTML(markdown)
	if err != nil {
		t.Fatalf("ToHTML failed: %v", err)
	}
	if !strings.Contains(html, "<code") {
		t.Errorf("expected <code> in output, got: %s", html)
	}
}

// ── Store package smoke test ────────────────────────────────────────

func TestStoreHashToken(t *testing.T) {
	h1 := store.HashToken("test-token-123")
	h2 := store.HashToken("test-token-123")
	if h1 != h2 {
		t.Errorf("hash should be deterministic: %q vs %q", h1, h2)
	}
	h3 := store.HashToken("different-token")
	if h1 == h3 {
		t.Errorf("different tokens should produce different hashes")
	}
	if len(h1) != 64 {
		t.Errorf("SHA-256 hex should be 64 chars, got %d", len(h1))
	}
}

func TestStoreNewInMemory(t *testing.T) {
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("New(:memory:) failed: %v", err)
	}

	// Create a new document
	id1, err := s.Create("# Test Doc\n\ncontent", store.HashToken("tok1"), store.HashToken("127.0.0.1"))
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if len(id1) != 8 {
		t.Errorf("expected 8-char ID, got %q (len=%d)", id1, len(id1))
	}

	// Retrieve it
	doc, err := s.GetByID(id1)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if doc == nil {
		t.Fatal("expected document, got nil")
	}
	if !strings.Contains(doc.Content, "# Test Doc") {
		t.Errorf("expected 'Test Doc' in content, got: %s", doc.Content)
	}
	if doc.CreatedAt == 0 {
		t.Error("expected non-zero CreatedAt")
	}

	// List by token
	docs, err := s.ListByTokenHash(store.HashToken("tok1"))
	if err != nil {
		t.Fatalf("ListByTokenHash failed: %v", err)
	}
	if len(docs) < 1 {
		t.Error("expected at least 1 document in list")
	}

	// Delete with wrong token
	deleted, err := s.Delete(id1, store.HashToken("wrong-token"))
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if deleted {
		t.Error("delete with wrong token should return false")
	}

	// Delete with right token
	deleted, err = s.Delete(id1, store.HashToken("tok1"))
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if !deleted {
		t.Error("delete with right token should return true")
	}

	// Verify deleted
	doc, err = s.GetByID(id1)
	if err != nil {
		t.Fatalf("GetByID after delete failed: %v", err)
	}
	if doc != nil {
		t.Error("expected nil after delete")
	}
}
