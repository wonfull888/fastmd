package main

import (
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/wonfull888/fastmd/internal/render"
	"github.com/wonfull888/fastmd/internal/store"
)

// Version is injected at build time via -ldflags.
var Version = "dev"

// TemplateRenderer wraps Go's html/template for Echo.
type TemplateRenderer struct {
	templates *template.Template
}

func (t *TemplateRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
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

	// Templates (load all HTML files from web/templates/)
	tmpl, err := template.ParseGlob("web/templates/*.html")
	if err != nil {
		log.Fatalf("failed to parse templates: %v", err)
	}

	// Echo
	e := echo.New()
	e.HideBanner = true
	e.Renderer = &TemplateRenderer{templates: tmpl}

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.BodyLimit("1MB"))
	e.Use(middleware.CORS())

	// Static files
	e.Static("/static", "web/static")

	// ── Routes ──

	// Homepage
	e.GET("/", func(c echo.Context) error {
		return c.Render(http.StatusOK, "base.html", map[string]interface{}{
			"Title":       "fastmd — Markdown pipeline for AI Agents",
			"Description": "Push Markdown from your terminal, get a shareable link instantly. No sign-up required.",
		})
	})

	// Docs page
	e.GET("/docs", func(c echo.Context) error {
		return c.Render(http.StatusOK, "base.html", map[string]interface{}{
			"Title":       "API & CLI Reference — fastmd",
			"Description": "Complete API and CLI reference for fastmd.",
		})
	})

	// Help page
	e.GET("/help", func(c echo.Context) error {
		return c.Render(http.StatusOK, "base.html", map[string]interface{}{
			"Title":       "Help & FAQ — fastmd",
			"Description": "Common questions about fastmd.",
		})
	})

	// install.sh
	e.GET("/install.sh", func(c echo.Context) error {
		b, err := os.ReadFile("install.sh")
		if err != nil {
			return c.String(http.StatusNotFound, "install.sh not found")
		}
		c.Response().Header().Set("Content-Type", "text/plain")
		return c.String(http.StatusOK, string(b))
	})

	// GET /v1/version
	e.GET("/v1/version", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"version":     Version,
			"install_url": "https://fastmd.dev/install.sh",
		})
	})

	// POST /v1/push — Create document
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
		ipHash := store.HashToken(c.RealIP()) // hash IP for abuse tracking

		id, err := db.Create(req.Content, tokenHash, ipHash)
		if err != nil {
			log.Printf("create error: %v", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}

		scheme := "https"
		if c.Request().TLS == nil {
			scheme = "http"
		}
		host := c.Request().Host
		url := fmt.Sprintf("%s://%s/%s", scheme, host, id)

		return c.JSON(http.StatusOK, map[string]string{
			"id":  id,
			"url": url,
		})
	})

	// DELETE /v1/:id — Delete document
	e.DELETE("/v1/:id", func(c echo.Context) error {
		id := c.Param("id")
		auth := c.Request().Header.Get("Authorization")
		token := strings.TrimPrefix(auth, "Bearer ")
		if token == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authorization required"})
		}

		tokenHash := store.HashToken(token)
		deleted, err := db.Delete(id, tokenHash)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}
		if !deleted {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "not found or token mismatch"})
		}
		return c.JSON(http.StatusOK, map[string]bool{"ok": true})
	})

	// GET /:id or GET /:id.md — View document (dual mode)
	e.GET("/:id", func(c echo.Context) error {
		id := c.Param("id")
		rawMode := false

		// Machine mode: .md suffix
		if strings.HasSuffix(id, ".md") {
			id = strings.TrimSuffix(id, ".md")
			rawMode = true
		}
		// Machine mode: Accept header
		if accept := c.Request().Header.Get("Accept"); accept == "text/plain" {
			rawMode = true
		}

		doc, err := db.GetByID(id)
		if err != nil {
			log.Printf("getbyid error: %v", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}
		if doc == nil {
			if rawMode {
				return c.String(http.StatusNotFound, "not found")
			}
			return c.Render(http.StatusNotFound, "base.html", map[string]interface{}{
				"Title":       "Not Found — fastmd",
				"Description": "Document not found.",
			})
		}

		if rawMode {
			c.Response().Header().Set("Content-Type", "text/plain; charset=utf-8")
			return c.String(http.StatusOK, doc.Content)
		}

		// Human mode: render Markdown → HTML
		htmlContent, err := render.ToHTML(doc.Content)
		if err != nil {
			htmlContent = "<pre>" + doc.Content + "</pre>"
		}

		return c.Render(http.StatusOK, "base.html", map[string]interface{}{
			"Title":       "fastmd/" + id,
			"Description": "Shared Markdown document on fastmd.",
			"ID":          id,
			"HTML":        template.HTML(htmlContent),
		})
	})

	log.Printf("fastmd %s starting on :%s", Version, *port)
	e.Logger.Fatal(e.Start(":" + *port))
}
