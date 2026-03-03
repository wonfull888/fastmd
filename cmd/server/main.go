package main

import (
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	fastmd "github.com/wonfull888/fastmd"
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
	pages := []string{"index", "doc", "404"}
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

func renderPage(c echo.Context, status int, page string, data map[string]interface{}) error {
	tmpl, ok := pageTemplates[page]
	if !ok {
		return fmt.Errorf("template not found: %s", page)
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
	port := flag.String("port", "8080", "Server port")
	dbPath := flag.String("db", "fastmd.db", "SQLite database path")
	flag.Parse()

	// Database
	db, err := store.New(*dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}

	// Templates (from embedded FS)
	if err := loadTemplates(); err != nil {
		log.Fatalf("failed to load templates: %v", err)
	}

	// Echo
	e := echo.New()
	e.HideBanner = true

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.BodyLimit("1MB"))
	e.Use(middleware.CORS())

	// Static files (from embedded FS)
	staticFS, err := fs.Sub(fastmd.WebFS, "web/static")
	if err != nil {
		log.Fatalf("failed to sub static FS: %v", err)
	}
	e.GET("/static/*", echo.WrapHandler(
		http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))),
	))

	// ── Routes ──

	e.GET("/", func(c echo.Context) error {
		title := "fastmd.dev | Minimalist CLI-First Markdown Hosting for Developers & AI Agents"
		description := "Instantly pipe Markdown from terminal to web. fastmd.dev is a minimalist, CLI-first hosting service for developers and AI agents. Anonymous, fast, and no-indexed by default."
		ogTitle := "fastmd.dev | The Markdown Pipe for CLI & AI"
		ogDescription := "Share Markdown in milliseconds. No login. AI-native dual view. Hard-coded for privacy."
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

	// POST /v1/push
	e.POST("/v1/push", func(c echo.Context) error {
		var req struct {
			Content string `json:"content"`
			Token   string `json:"token"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		if strings.TrimSpace(req.Content) == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "content is empty"})
		}
		if strings.TrimSpace(req.Token) == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "token is required"})
		}

		tokenHash := store.HashToken(req.Token)
		ipHash := store.HashToken(c.RealIP())

		id, err := db.Create(req.Content, tokenHash, ipHash)
		if err != nil {
			log.Printf("create error: %v", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}

		url := absoluteURL(c, "/"+id)

		return c.JSON(http.StatusOK, map[string]string{"id": id, "url": url})
	})

	// DELETE /v1/:id
	e.DELETE("/v1/:id", func(c echo.Context) error {
		id := c.Param("id")
		auth := c.Request().Header.Get("Authorization")
		token := strings.TrimPrefix(auth, "Bearer ")
		if token == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authorization required"})
		}
		deleted, err := db.Delete(id, store.HashToken(token))
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}
		if !deleted {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "not found or token mismatch"})
		}
		return c.JSON(http.StatusOK, map[string]bool{"ok": true})
	})

	// GET /:id  (dual mode: HTML or raw)
	e.GET("/:id", func(c echo.Context) error {
		idParam := c.Param("id")
		rawMode := strings.HasSuffix(idParam, ".md") ||
			c.Request().Header.Get("Accept") == "text/plain"
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

		if rawMode {
			c.Response().Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive")
			c.Response().Header().Set("Content-Type", "text/plain; charset=utf-8")
			return c.String(http.StatusOK, doc.Content)
		}

		htmlContent, err := render.ToHTML(doc.Content)
		if err != nil {
			htmlContent = "<pre>" + doc.Content + "</pre>"
		}
		canonical := absoluteURL(c, "/"+id)
		c.Response().Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive")
		return renderPage(c, http.StatusOK, "doc", map[string]interface{}{
			"Title":              "fastmd/" + id + " | fastmd.dev",
			"Description":        "Shared Markdown document on fastmd.",
			"Canonical":          canonical,
			"Robots":             "noindex, nofollow, noarchive",
			"OGType":             "article",
			"OGURL":              canonical,
			"OGImage":            "https://fastmd.dev/static/og-fastmd.svg",
			"TwitterCard":        "summary",
			"TwitterDescription": "Shared Markdown document on fastmd.",
			"TwitterImage":       "https://fastmd.dev/static/og-fastmd.svg",
			"ID":                 id,
			"HTML":               template.HTML(htmlContent),
		})
	})

	log.Printf("fastmd %s starting on :%s", Version, *port)
	e.Logger.Fatal(e.Start(":" + *port))
}
