package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// Version is injected at build time.
var Version = "dev"

var baseURL = "https://fastmd.dev"

var httpClient = newHTTPClient()

// Some networks accept IPv6 TCP connections but stall during TLS. Prefer IPv4
// before sending any HTTP request, while retaining IPv6-only connectivity.
func newHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		if network == "tcp" {
			if conn, err := dialer.DialContext(ctx, "tcp4", address); err == nil {
				return conn, nil
			}
		}
		return dialer.DialContext(ctx, network, address)
	}
	return &http.Client{Transport: transport, Timeout: 30 * time.Second}
}

const (
	configDir = ".config/fastmd"
	tokenFile = "token"
	tokenPfx  = "fmd_live_"
)

func main() {
	if endpoint := os.Getenv("FASTMD_BASE_URL"); endpoint != "" {
		baseURL = strings.TrimRight(endpoint, "/")
	}
	parseGlobalArgs()
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `fastmd %s — Markdown pipeline for AI Agents & developers

Usage:
  cat file.md | fastmd          Push via pipe
  fastmd push <file>            Push a file
  fastmd get <ID>               Pull document to local file
  fastmd update <ID> <file>     Replace a document, keeping its URL
  fastmd delete <ID>            Delete a document
  fastmd list                   List your documents
  fastmd usage                  Show account and document count
  fastmd login                  Sign in with GitHub in your browser
  fastmd logout                 Revoke this login
  fastmd whoami                 Show current account
  --json / --server <origin>     Machine output / server selection
  fastmd upgrade                Upgrade to latest version
  fastmd --version              Print version

`, Version)
	}

	if len(os.Args) < 2 {
		// Check if stdin has data (pipe mode)
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			pushFromReader(os.Stdin)
			return
		}
		flag.Usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "--help", "-h", "help":
		flag.Usage()
	case "list", "usage", "login", "logout", "whoami", "claim":
		accountCommand(os.Args[1:])
	case "--version", "-v", "version":
		fmt.Printf("fastmd %s\n", Version)
	case "push":
		if len(os.Args) < 3 {
			die("Usage: fastmd push <file>")
		}
		f, err := os.Open(os.Args[2])
		must(err, "open file")
		defer f.Close()
		pushFromReader(f)
	case "get":
		if len(os.Args) < 3 {
			die("Usage: fastmd get <ID>")
		}
		cmdGet(os.Args[2])
	case "update":
		if len(os.Args) != 4 {
			die("Usage: fastmd update <ID> <file>")
		}
		if !regexp.MustCompile(`^[A-Za-z0-9]+$`).MatchString(os.Args[2]) {
			die("Error: use the document ID, without URL or .md suffix")
		}
		f, err := os.Open(os.Args[3])
		must(err, "open file")
		defer f.Close()
		cmdUpdate(os.Args[2], f)
	case "delete":
		if len(os.Args) < 3 {
			die("Usage: fastmd delete <ID>")
		}
		cmdDelete(os.Args[2])
	case "upgrade":
		cmdUpgrade()
	default:
		// Treat as file path
		f, err := os.Open(os.Args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
			flag.Usage()
			os.Exit(1)
		}
		defer f.Close()
		pushFromReader(f)
	}
}

// ── Push ──────────────────────────────────────────────────────────────────

func pushFromReader(r io.Reader) {
	content, err := io.ReadAll(r)
	must(err, "read input")
	if strings.TrimSpace(string(content)) == "" {
		die("Error: content is empty")
	}

	token := loadToken()

	body, _ := json.Marshal(map[string]string{
		"content":    string(content),
		"request_id": requestID,
	})

	req, err := http.NewRequest(http.MethodPost, baseURL+"/v1/push", bytes.NewReader(body))
	must(err, "create push request")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	must(err, "push request")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		handleHTTPError(resp.StatusCode, b)
	}

	var result struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	must(json.NewDecoder(resp.Body).Decode(&result), "decode response")
	if jsonOutput {
		encoded, _ := json.Marshal(result)
		printJSON(encoded)
	} else {
		fmt.Printf("✓  Published → %s\n", result.URL)
	}
}

// ── Update ────────────────────────────────────────────────────────────────

func cmdUpdate(id string, r io.Reader) {
	content, err := io.ReadAll(r)
	must(err, "read input")
	if strings.TrimSpace(string(content)) == "" {
		die("Error: content is empty")
	}
	token := loadToken()
	body, err := json.Marshal(map[string]string{"content": string(content)})
	must(err, "encode update")
	req, err := http.NewRequest(http.MethodPut, baseURL+"/v1/"+id, bytes.NewReader(body))
	must(err, "create update request")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := httpClient.Do(req)
	must(err, "update request")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		handleHTTPError(resp.StatusCode, b)
	}
	var result struct {
		URL string `json:"url"`
	}
	must(json.NewDecoder(resp.Body).Decode(&result), "decode response")
	if result.URL == "" {
		die("Error: server response did not include a URL")
	}
	if jsonOutput {
		encoded, _ := json.Marshal(map[string]string{"id": id, "url": result.URL})
		printJSON(encoded)
	} else {
		fmt.Printf("✓  Updated → %s\n", result.URL)
	}
}

// ── Get ───────────────────────────────────────────────────────────────────

func cmdGet(id string) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/%s.md", baseURL, id), nil)
	must(err, "create request")
	req.Header.Set("Accept", "text/plain")
	resp, err := httpClient.Do(req)
	must(err, "get request")
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		die("Error: document '%s' not found", id)
	}
	if resp.StatusCode != http.StatusOK {
		die("Error: server returned %d", resp.StatusCode)
	}

	content, err := io.ReadAll(resp.Body)
	must(err, "read response")

	if jsonOutput {
		encoded, _ := json.Marshal(map[string]string{"id": id, "url": baseURL + "/" + id + ".md", "content": string(content)})
		printJSON(encoded)
		return
	}
	filename := slugify(extractH1(string(content)))
	if filename == "" {
		filename = id
	}
	filename += ".md"

	must(os.WriteFile(filename, content, 0644), "write file")
	fmt.Printf("✓  Saved → %s\n", filename)
}

// ── Delete ────────────────────────────────────────────────────────────────

func cmdDelete(id string) {
	token := loadToken()

	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/v1/%s", baseURL, id), nil)
	must(err, "create request")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := httpClient.Do(req)
	must(err, "delete request")
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		if jsonOutput {
			printJSON([]byte(`{"ok":true,"id":"` + id + `"}`))
		} else {
			fmt.Printf("✓  Deleted: %s\n", id)
		}
	case http.StatusForbidden:
		die("Error: document not found or token mismatch")
	default:
		die("Error: server returned %d", resp.StatusCode)
	}
}

// ── Upgrade ───────────────────────────────────────────────────────────────

func cmdUpgrade() {
	fmt.Println("Upgrading fastmd...")
	cmd := exec.Command("sh", "-c", "curl -fsSL "+baseURL+"/install.sh | sh")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	must(cmd.Run(), "upgrade")
}

// ── Token ─────────────────────────────────────────────────────────────────

// ── Helpers ───────────────────────────────────────────────────────────────

var h1Re = regexp.MustCompile(`(?m)^#\s+(.+)$`)

func extractH1(content string) string {
	m := h1Re.FindStringSubmatch(content)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

func slugify(s string) string {
	s = strings.ToLower(s)
	var sb strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			sb.WriteRune(r)
		} else if r == ' ' || r == '-' {
			sb.WriteRune('-')
		}
	}
	slug := strings.Trim(sb.String(), "-")
	// Collapse multiple dashes
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	return slug
}

func must(err error, msg string) {
	if err != nil {
		apiDie("CLIENT_ERROR", fmt.Sprintf("%s: %v", msg, err))
	}
}

func die(format string, args ...interface{}) {
	apiDie("CLIENT_ERROR", fmt.Sprintf(format, args...))
}
