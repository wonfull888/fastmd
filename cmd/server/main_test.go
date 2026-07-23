package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

// ── Unit tests: extractDescription ──────────────────────────────────

func TestExtractDescription_ShortContent(t *testing.T) {
	content := "This is a simple document."
	got := extractDescription(content, 200)
	if got != "This is a simple document." {
		t.Errorf("expected full text, got %q", got)
	}
}

func TestExtractDescription_TruncatedWithWordBoundary(t *testing.T) {
	// Build a string longer than 200 chars
	prefix := strings.Repeat("word ", 50) // 250 chars
	content := prefix + "and then some more text"
	got := extractDescription(content, 200)
	if len(got) > 210 {
		t.Errorf("expected <= 210 chars, got %d: %q", len(got), got)
	}
	// Should not end mid-word
	if len(got) > 0 && got[len(got)-1] != ' ' {
		// Last char is not space — but it could be a complete word boundary
		// Just check it doesn't cut in the middle of "word"
		if strings.HasSuffix(got, "wor") {
			t.Errorf("appears to cut mid-word: %q", got)
		}
	}
}

func TestExtractDescription_SkipsCodeBlocks(t *testing.T) {
	content := "```go\nfunc main() {}\n```\n\nThis is the summary text."
	got := extractDescription(content, 200)
	if strings.Contains(got, "func main()") {
		t.Errorf("code block content leaked into description: %q", got)
	}
	if !strings.Contains(got, "This is the summary text") {
		t.Errorf("expected summary text, got %q", got)
	}
}

func TestExtractDescription_SkipsHeadings(t *testing.T) {
	content := "# Big Title\n\n## Subtitle\n\nActual paragraph text here."
	got := extractDescription(content, 200)
	if strings.Contains(got, "Big Title") || strings.Contains(got, "Subtitle") {
		t.Errorf("headings leaked into description: %q", got)
	}
	if !strings.Contains(got, "Actual paragraph text here") {
		t.Errorf("expected paragraph text, got %q", got)
	}
}

func TestExtractDescription_StripsLinks(t *testing.T) {
	content := "Check [the docs](https://example.com) for more info."
	got := extractDescription(content, 200)
	if strings.Contains(got, "https://example.com") || strings.Contains(got, "](") {
		t.Errorf("link syntax not stripped: %q", got)
	}
	if !strings.Contains(got, "the docs") {
		t.Errorf("link text should be preserved: %q", got)
	}
}

func TestExtractDescription_StripsImages(t *testing.T) {
	content := "![screenshot](img.png) Welcome to the guide."
	got := extractDescription(content, 200)
	if strings.Contains(got, "!") || strings.Contains(got, "img.png") {
		t.Errorf("image syntax not stripped: %q", got)
	}
	if !strings.Contains(got, "Welcome to the guide") {
		t.Errorf("expected subsequent text, got %q", got)
	}
}

func TestExtractDescription_StripsFormatting(t *testing.T) {
	content := "This is **bold** and _italic_ and `code`."
	got := extractDescription(content, 200)
	for _, marker := range []string{"**", "__", "`", "_"} {
		if strings.Contains(got, marker) {
			t.Errorf("formatting marker %q not stripped: %q", marker, got)
		}
	}
}

func TestExtractDescription_EmptyContent(t *testing.T) {
	if got := extractDescription("", 200); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
	if got := extractDescription("# Only heading", 200); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestExtractDescription_VeryShort(t *testing.T) {
	got := extractDescription("Hi", 200)
	if got != "Hi" {
		t.Errorf("expected 'Hi', got %q", got)
	}
}

// ── Unit tests: extractTitle ────────────────────────────────────────

func TestExtractTitle_Found(t *testing.T) {
	content := "# Hello World\n\nSome content."
	got := extractTitle(content, "fallback")
	if got != "Hello World" {
		t.Errorf("expected 'Hello World', got %q", got)
	}
}

func TestExtractTitle_NotFound(t *testing.T) {
	content := "No title here\nJust content."
	got := extractTitle(content, "fallback")
	if got != "fallback" {
		t.Errorf("expected fallback, got %q", got)
	}
}

func TestExtractTitle_SkipsH2(t *testing.T) {
	content := "## Subtitle\n\n# Real Title"
	got := extractTitle(content, "fallback")
	if got != "Real Title" {
		t.Errorf("expected 'Real Title', got %q", got)
	}
}

// ── Unit tests: stripMarkdown ───────────────────────────────────────

func TestStripMarkdown_Blockquote(t *testing.T) {
	got := stripMarkdown("> This is a quote")
	if strings.HasPrefix(got, ">") {
		t.Errorf("blockquote marker not stripped: %q", got)
	}
	if got != "This is a quote" {
		t.Errorf("expected 'This is a quote', got %q", got)
	}
}

// ── Unit tests: formatCreatedAt ─────────────────────────────────────

func TestFormatCreatedAt_Zero(t *testing.T) {
	if got := formatCreatedAt(0); got != "" {
		t.Errorf("expected empty for zero ts, got %q", got)
	}
}

func TestFormatCreatedAt_Valid(t *testing.T) {
	// 2026-07-23 12:00:00 UTC
	got := formatCreatedAt(1784808000)
	if !strings.Contains(got, "2026-07-23") {
		t.Errorf("expected '2026-07-23' in result, got %q", got)
	}
	if !strings.Contains(got, "UTC") {
		t.Errorf("expected 'UTC' in result, got %q", got)
	}
}

// ── Integration tests: .md suffix in API responses ──────────────────

func TestAbsoluteURL(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	got := absoluteURL(c, "/abc123.md")
	if !strings.HasSuffix(got, ".md") {
		t.Errorf("expected .md suffix, got %q", got)
	}
}

func TestExtractDescription_HTMLSpecialChars(t *testing.T) {
	content := "This has \"quotes\" and & ampersand."
	got := extractDescription(content, 200)
	// Go html/template will escape these in actual rendering
	if !strings.Contains(got, "\"quotes\"") {
		t.Errorf("expected quotes preserved in raw string, got %q", got)
	}
	if !strings.Contains(got, "& ampersand") {
		t.Errorf("expected ampersand in raw string, got %q", got)
	}
}
