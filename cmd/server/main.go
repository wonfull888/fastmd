package main

import (
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	fastmd "github.com/wonfull888/fastmd"
	"github.com/wonfull888/fastmd/internal/app"
	"github.com/wonfull888/fastmd/internal/config"
	"github.com/wonfull888/fastmd/internal/render"
	"github.com/wonfull888/fastmd/internal/store"
)

// Version is injected at build time via -ldflags.
var Version = "dev"

// pageTemplates holds a separate template set per page.
var pageTemplates map[string]*template.Template

const homeJSONLD = `{
  "@context": "https://schema.org",
  "@graph": [
    {
      "@type": "Organization",
      "name": "fastmd.dev",
      "url": "https://fastmd.dev",
      "logo": "https://fastmd.dev/static/og-fastmd.svg"
    },
    {
      "@type": "SoftwareApplication",
      "name": "fastmd",
      "url": "https://fastmd.dev",
      "applicationCategory": "DeveloperApplication",
      "operatingSystem": "Linux, macOS, Windows",
      "description": "A minimalist CLI-first Markdown hosting and sync pipe for AI Agents and developers.",
      "offers": {
        "@type": "Offer",
        "price": "0",
        "priceCurrency": "USD"
      },
      "sameAs": [
        "https://github.com/wonfull888/fastmd"
      ]
    },
    {
      "@type": "FAQPage",
      "mainEntity": [
        {
          "@type": "Question",
          "name": "Where is my token stored?",
          "acceptedAnswer": {
            "@type": "Answer",
            "text": "On first run, fastmd creates a token and stores it at ~/.config/fastmd/token."
          }
        },
        {
          "@type": "Question",
          "name": "What happens if I lose my token?",
          "acceptedAnswer": {
            "@type": "Answer",
            "text": "Documents remain readable, but deletion is no longer possible without the original token."
          }
        },
        {
          "@type": "Question",
          "name": "What is the document size limit?",
          "acceptedAnswer": {
            "@type": "Answer",
            "text": "The current payload limit is 1MB per document."
          }
        },
        {
          "@type": "Question",
          "name": "Do documents expire?",
          "acceptedAnswer": {
            "@type": "Answer",
            "text": "In v0.1, documents are stored permanently by default."
          }
        },
        {
          "@type": "Question",
          "name": "How is token privacy handled?",
          "acceptedAnswer": {
            "@type": "Answer",
            "text": "The token is stored locally only, and the backend stores only its hash. Raw token values are never stored."
          }
        },
        {
          "@type": "Question",
          "name": "How do agents get raw markdown?",
          "acceptedAnswer": {
            "@type": "Answer",
            "text": "Call https://fastmd.dev/{id}.md or use Accept: text/plain."
          }
        },
        {
          "@type": "Question",
          "name": "Can I self-host fastmd.dev?",
          "acceptedAnswer": {
            "@type": "Answer",
            "text": "Yes. fastmd is open-source (MIT), built as a single Go server with SQLite storage."
          }
        }
      ]
    }
  ]
}`

func loadTemplates() error {
	pages := []string{"index", "doc", "404", "dashboard", "md-hint"}
	pageTemplates = make(map[string]*template.Template, len(pages))
	for _, name := range pages {
		t, err := template.ParseFS(
			fastmd.WebFS,
			"web/templates/base.html",
			"web/templates/"+name+".html",
		)
		if err != nil {
			return fmt.Errorf("parse template %s: %w", name, err)
		}
		pageTemplates[name] = t
	}
	return nil
}

// formatCreatedAt converts a Unix timestamp to "2006-01-02 15:04 UTC".
// Returns "" if ts is 0 (old documents with no recorded creation time).
func formatCreatedAt(ts int64) string {
	if ts == 0 {
		return ""
	}
	return time.Unix(ts, 0).UTC().Format("2006-01-02 15:04 UTC")
}

type rateClient struct {
	windowStart time.Time
	count       int
}

type ipRateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	clients map[string]*rateClient
}

func newIPRateLimiter(limit int, window time.Duration) *ipRateLimiter {
	return &ipRateLimiter{
		limit:   limit,
		window:  window,
		clients: make(map[string]*rateClient),
	}
}

func (l *ipRateLimiter) allow(ip string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	for key, client := range l.clients {
		if now.Sub(client.windowStart) >= l.window {
			delete(l.clients, key)
		}
	}

	client, ok := l.clients[ip]
	if !ok {
		l.clients[ip] = &rateClient{windowStart: now, count: 1}
		return true
	}

	if now.Sub(client.windowStart) >= l.window {
		client.windowStart = now
		client.count = 1
		return true
	}

	if client.count >= l.limit {
		return false
	}

	client.count++
	return true
}

func extractTitle(content, fallback string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			title := strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
			if title != "" {
				return title
			}
		}
	}
	return fallback
}

// Pre-compiled patterns for stripMarkdown.
var (
	imgRe  = regexp.MustCompile(`!\[[^\]]*\]\([^)]+\)`)
	linkRe = regexp.MustCompile(`\[([^\]]*)\]\([^)]+\)`)
)

// stripMarkdown removes common markdown formatting from a single line.
func stripMarkdown(line string) string {
	line = imgRe.ReplaceAllString(line, "")
	line = linkRe.ReplaceAllString(line, "$1")
	line = strings.ReplaceAll(line, "**", "")
	line = strings.ReplaceAll(line, "__", "")
	line = strings.ReplaceAll(line, "*", "")
	line = strings.ReplaceAll(line, "_", "")
	line = strings.ReplaceAll(line, "`", "")
	line = strings.TrimPrefix(line, "> ")
	return strings.TrimSpace(line)
}

// extractDescription returns a plain-text summary of up to maxChars characters
// from the beginning of markdown content. It strips markdown syntax and extends
// to the next space so words are never cut in the middle.
func extractDescription(content string, maxChars int) string {
	var lines []string
	inCodeBlock := false

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			continue
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		clean := stripMarkdown(trimmed)
		if clean != "" {
			lines = append(lines, clean)
		}
	}

	text := strings.Join(lines, " ")
	text = strings.Join(strings.Fields(text), " ")

	if len(text) <= maxChars {
		return text
	}

	truncated := text[:maxChars]
	if idx := strings.LastIndexByte(truncated, ' '); idx > 0 {
		truncated = truncated[:idx]
	}
	return truncated
}

func renderPage(c echo.Context, status int, page string, data map[string]interface{}) error {
	tmpl, ok := pageTemplates[page]
	if !ok {
		return fmt.Errorf("template not found: %s", page)
	}
	if data == nil {
		data = map[string]interface{}{}
	}
	if _, exists := data["AssetVersion"]; !exists {
		data["AssetVersion"] = Version
	}
	c.Response().WriteHeader(status)
	c.Response().Header().Set("Content-Type", "text/html; charset=utf-8")
	return tmpl.ExecuteTemplate(c.Response().Writer, "base.html", data)
}

func requestScheme(c echo.Context) string {
	if c.Request().TLS != nil || c.Request().Header.Get("X-Forwarded-Proto") == "https" {
		return "https"
	}
	return "http"
}

func absoluteURL(c echo.Context, path string) string {
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return fmt.Sprintf("%s://%s%s", requestScheme(c), c.Request().Host, path)
}

func main() {
	if err := config.LoadEnv(".env"); err != nil {
		log.Fatal(err)
	}
	port := flag.String("port", "8080", "Server port")
	host := flag.String("host", "127.0.0.1", "Listen address")
	publicURL := flag.String("public-url", os.Getenv("FASTMD_PUBLIC_URL"), "Public HTTP(S) origin")
	dbPath := flag.String("db", "fastmd.db", "SQLite database path")
	flag.Parse()

	// Database
	db, err := store.New(*dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}

	if *publicURL == "" {
		*publicURL = "http://" + net.JoinHostPort(*host, *port)
	}
	github := app.GitHubConfig{ClientID: os.Getenv("GITHUB_CLIENT_ID"), ClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"), RedirectURI: os.Getenv("GITHUB_REDIRECT_URI")}
	if (github.ClientID == "") != (github.ClientSecret == "") {
		log.Fatal("both GITHUB_CLIENT_ID and GITHUB_CLIENT_SECRET are required")
	}
	if github.RedirectURI != "" && github.RedirectURI != strings.TrimRight(*publicURL, "/")+"/auth/github/callback" {
		log.Fatal("GITHUB_REDIRECT_URI must match FASTMD_PUBLIC_URL/auth/github/callback")
	}
	e, err := newServerWithOptions(db, app.Options{PublicURL: *publicURL, GitHub: github, MCPOrigins: strings.Fields(os.Getenv("FASTMD_MCP_ORIGINS"))})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("fastmd %s starting on :%s", Version, *port)
	e.Server.ReadHeaderTimeout = 10 * time.Second
	e.Server.IdleTimeout = 60 * time.Second
	e.Logger.Fatal(e.Start(net.JoinHostPort(*host, *port)))
}

func newServer(db *store.Store) (*echo.Echo, error) {
	return newServerWithOptions(db, app.Options{PublicURL: "http://example.com"})
}

func newServerWithOptions(db *store.Store, options app.Options) (*echo.Echo, error) {
	// Templates (from embedded FS)
	if err := loadTemplates(); err != nil {
		return nil, fmt.Errorf("failed to load templates: %w", err)
	}

	// Echo
	e := echo.New()
	e.HideBanner = true

	// Middleware
	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{Format: "${method} ${path} ${status}\n"}))
	e.Use(middleware.SecureWithConfig(middleware.SecureConfig{XFrameOptions: "DENY", ContentTypeNosniff: "nosniff", ContentSecurityPolicy: "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' https: data:; object-src 'none'; base-uri 'self'; frame-ancestors 'none'"}))
	e.Use(middleware.Recover())
	e.Use(middleware.BodyLimit("1MB"))
	e.Use(middleware.CORS())
	rateLimiter := newIPRateLimiter(60, time.Minute)
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			path := c.Request().URL.Path
			if strings.HasPrefix(path, "/static/") {
				return next(c)
			}
			if !rateLimiter.allow(c.RealIP()) {
				if strings.HasPrefix(path, "/v1/") {
					return c.JSON(http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
				}
				return c.String(http.StatusTooManyRequests, "rate limit exceeded")
			}
			return next(c)
		}
	})

	// Static files (from embedded FS)
	staticFS, err := fs.Sub(fastmd.WebFS, "web/static")
	if err != nil {
		return nil, fmt.Errorf("failed to sub static FS: %w", err)
	}
	e.GET("/static/*", echo.WrapHandler(
		http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))),
	))

	// ── Routes ──

	e.GET("/", func(c echo.Context) error {
		title := "fastmd.dev | Minimalist CLI-First Markdown Hosting for Developers & AI Agents"
		description := "Instantly pipe Markdown from terminal to web. fastmd.dev is a minimalist, CLI-first hosting service for developers and AI agents. Anonymous, fast, and no-indexed by default."
		ogTitle := "fastmd.dev | The Markdown Pipe for CLI & AI"
		ogDescription := "Share Markdown in milliseconds. Built for AI agents, with GitHub sign-in and CLI or MCP access."
		canonical := "https://fastmd.dev/"
		ogImage := "https://fastmd.dev/static/og-fastmd.svg"

		return renderPage(c, http.StatusOK, "index", map[string]interface{}{
			"Title":              title,
			"Description":        description,
			"Canonical":          canonical,
			"Robots":             "index, follow, max-image-preview:large",
			"OGTitle":            ogTitle,
			"OGDescription":      ogDescription,
			"OGType":             "website",
			"OGURL":              canonical,
			"OGImage":            ogImage,
			"TwitterCard":        "summary_large_image",
			"TwitterTitle":       ogTitle,
			"TwitterDescription": ogDescription,
			"TwitterImage":       ogImage,
			"JSONLD":             template.JS(homeJSONLD),
		})
	})

	// install.sh — serve from embedded binary
	e.GET("/install.sh", func(c echo.Context) error {
		c.Response().Header().Set("Content-Type", "text/plain")
		return c.String(http.StatusOK, string(fastmd.InstallSH))
	})

	e.GET("/v1/version", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"version":     Version,
			"install_url": "https://fastmd.dev/install.sh",
		})
	})

	accountApp, err := app.Register(e, db, options)
	if err != nil {
		return nil, err
	}
	e.GET("/login", func(c echo.Context) error { return accountApp.LoginPage(c, c.QueryParam("next")) })

	e.GET("/dashboard", func(c echo.Context) error {
		if _, err := accountApp.Session(c); err != nil {
			return accountApp.LoginPage(c, "/dashboard")
		}
		c.Response().Header().Set("Cache-Control", "no-store")
		canonical := absoluteURL(c, "/dashboard")
		return renderPage(c, http.StatusOK, "dashboard", map[string]interface{}{
			"Title":              "Dashboard | fastmd.dev",
			"Description":        "Manage documents published with your fastmd token.",
			"Canonical":          canonical,
			"Robots":             "noindex, nofollow, noarchive",
			"OGType":             "website",
			"OGURL":              canonical,
			"OGImage":            "https://fastmd.dev/static/og-fastmd.svg",
			"TwitterCard":        "summary",
			"TwitterDescription": "Manage documents published with your fastmd token.",
			"TwitterImage":       "https://fastmd.dev/static/og-fastmd.svg",
		})
	})

	// GET /:id  (triple mode: doc HTML / md-hint HTML / raw text)
	e.GET("/:id", func(c echo.Context) error {
		c.Response().Header().Set("Cache-Control", "no-cache")
		idParam := c.Param("id")
		isMdSuffix := strings.HasSuffix(idParam, ".md")
		acceptPlain := c.Request().Header.Get("Accept") == "text/plain"
		rawMode := acceptPlain                   // M-4: Accept: text/plain always returns raw (backward compat)
		mdHintMode := isMdSuffix && !acceptPlain // M-3: .md without text/plain → HTML hint page
		id := strings.TrimSuffix(idParam, ".md")

		doc, err := db.GetByID(id)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}
		if doc == nil {
			if rawMode {
				c.Response().Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive")
				return c.String(http.StatusNotFound, "not found")
			}
			c.Response().Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive")
			return renderPage(c, http.StatusNotFound, "404", map[string]interface{}{
				"Title":        "Document not found | fastmd.dev",
				"Description":  "This shared Markdown document does not exist or has been deleted.",
				"Canonical":    absoluteURL(c, c.Request().URL.Path),
				"Robots":       "noindex, nofollow, noarchive",
				"OGType":       "website",
				"OGURL":        absoluteURL(c, c.Request().URL.Path),
				"OGImage":      "https://fastmd.dev/static/og-fastmd.svg",
				"TwitterImage": "https://fastmd.dev/static/og-fastmd.svg",
			})
		}

		// M-4: raw text/plain — backward compat for agents
		if rawMode {
			c.Response().Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive")
			c.Response().Header().Set("Content-Type", "text/plain; charset=utf-8")
			return c.String(http.StatusOK, doc.Content)
		}

		// Compute dynamic OG fields
		docTitle := extractTitle(doc.Content, "")
		ogTitle := docTitle
		if ogTitle == "" {
			ogTitle = "fastmd/" + id + " | fastmd.dev"
		}
		pageTitle := ogTitle
		if docTitle != "" {
			pageTitle = docTitle + " | fastmd.dev"
		}
		ogDesc := extractDescription(doc.Content, 200)
		if ogDesc == "" {
			ogDesc = "Shared Markdown document on fastmd."
		}
		canonical := absoluteURL(c, "/"+id)
		ogImage := "https://fastmd.dev/static/og-fastmd.svg"
		robots := "noindex, nofollow, noarchive"

		// M-3: .md HTML hint page
		if mdHintMode {
			c.Response().Header().Set("X-Robots-Tag", robots)
			return renderPage(c, http.StatusOK, "md-hint", map[string]interface{}{
				"Title":              pageTitle,
				"Description":        ogDesc,
				"Canonical":          canonical + ".md",
				"Robots":             robots,
				"OGTitle":            ogTitle,
				"OGDescription":      ogDesc,
				"OGType":             "article",
				"OGURL":              canonical + ".md",
				"OGImage":            ogImage,
				"TwitterCard":        "summary",
				"TwitterTitle":       ogTitle,
				"TwitterDescription": ogDesc,
				"TwitterImage":       ogImage,
				"ID":                 id,
				"DocTitle":           docTitle,
				"RawContent":         doc.Content,
				"CreatedAt":          formatCreatedAt(doc.CreatedAt),
			})
		}

		// M-5, M-6: rendered doc page with dynamic OG
		htmlContent, err := render.ToHTML(doc.Content)
		if err != nil {
			htmlContent = "<pre>" + template.HTMLEscapeString(doc.Content) + "</pre>"
		}
		c.Response().Header().Set("X-Robots-Tag", robots)
		return renderPage(c, http.StatusOK, "doc", map[string]interface{}{
			"Title":              pageTitle,
			"Description":        ogDesc,
			"Canonical":          canonical,
			"Robots":             robots,
			"OGTitle":            ogTitle,
			"OGDescription":      ogDesc,
			"OGType":             "article",
			"OGURL":              canonical,
			"OGImage":            ogImage,
			"TwitterCard":        "summary",
			"TwitterTitle":       ogTitle,
			"TwitterDescription": ogDesc,
			"TwitterImage":       ogImage,
			"ID":                 id,
			"HTML":               template.HTML(htmlContent),
			"CreatedAt":          formatCreatedAt(doc.CreatedAt),
		})
	})

	return e, nil
}
