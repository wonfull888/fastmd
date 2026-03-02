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

func loadTemplates() error {
	pages := []string{"index", "doc", "docs", "help", "404"}
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
		return renderPage(c, http.StatusOK, "index", map[string]interface{}{
			"Title":       "fastmd — Markdown pipeline for AI Agents",
			"Description": "Push Markdown from your terminal, get a shareable link instantly. No sign-up required.",
		})
	})

	e.GET("/docs", func(c echo.Context) error {
		return renderPage(c, http.StatusOK, "docs", map[string]interface{}{
			"Title":       "API & CLI Reference — fastmd",
			"Description": "Complete API and CLI reference for fastmd.",
		})
	})

	e.GET("/help", func(c echo.Context) error {
		return renderPage(c, http.StatusOK, "help", map[string]interface{}{
			"Title":       "Help & FAQ — fastmd",
			"Description": "Common questions about fastmd.",
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

		scheme := "https"
		if c.Request().TLS == nil && c.Request().Header.Get("X-Forwarded-Proto") != "https" {
			scheme = "http"
		}
		url := fmt.Sprintf("%s://%s/%s", scheme, c.Request().Host, id)

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
		id := c.Param("id")
		rawMode := strings.HasSuffix(id, ".md") ||
			c.Request().Header.Get("Accept") == "text/plain"
		id = strings.TrimSuffix(id, ".md")

		doc, err := db.GetByID(id)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}
		if doc == nil {
			if rawMode {
				return c.String(http.StatusNotFound, "not found")
			}
			return renderPage(c, http.StatusNotFound, "404", map[string]interface{}{
				"Title": "Not Found — fastmd",
			})
		}

		if rawMode {
			c.Response().Header().Set("Content-Type", "text/plain; charset=utf-8")
			return c.String(http.StatusOK, doc.Content)
		}

		htmlContent, err := render.ToHTML(doc.Content)
		if err != nil {
			htmlContent = "<pre>" + doc.Content + "</pre>"
		}
		return renderPage(c, http.StatusOK, "doc", map[string]interface{}{
			"Title":       "fastmd/" + id,
			"Description": "Shared Markdown document on fastmd.",
			"ID":          id,
			"HTML":        template.HTML(htmlContent),
		})
	})

	log.Printf("fastmd %s starting on :%s", Version, *port)
	e.Logger.Fatal(e.Start(":" + *port))
}
