package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// Version is injected at build time.
var Version = "dev"

const (
	baseURL    = "https://fastmd.dev"
	configDir  = ".config/fastmd"
	tokenFile  = "token"
	tokenPfx   = "fmd_live_"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `fastmd %s — Markdown pipeline for AI Agents & developers

Usage:
  cat file.md | fastmd          Push via pipe
  fastmd push <file>            Push a file
  fastmd get <ID>               Pull document to local file
  fastmd delete <ID>            Delete a document
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

	token := loadOrCreateToken()

	body, _ := json.Marshal(map[string]string{
		"content": string(content),
		"token":   token,
	})

	resp, err := http.Post(baseURL+"/v1/push", "application/json", bytes.NewReader(body))
	must(err, "push request")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		die("Error: server returned %d: %s", resp.StatusCode, string(b))
	}

	var result struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	must(json.NewDecoder(resp.Body).Decode(&result), "decode response")
	fmt.Printf("✓  Published → %s\n", result.URL)
}

// ── Get ───────────────────────────────────────────────────────────────────

func cmdGet(id string) {
	resp, err := http.Get(fmt.Sprintf("%s/%s.md", baseURL, id))
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
	token := loadOrCreateToken()

	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/v1/%s", baseURL, id), nil)
	must(err, "create request")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	must(err, "delete request")
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		fmt.Printf("✓  Deleted: %s\n", id)
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

func loadOrCreateToken() string {
	home, err := os.UserHomeDir()
	must(err, "get home dir")

	dir := filepath.Join(home, configDir)
	path := filepath.Join(dir, tokenFile)

	data, err := os.ReadFile(path)
	if err == nil {
		token := strings.TrimSpace(string(data))
		if token != "" {
			return token
		}
	}

	// Generate new token
	token := tokenPfx + randString(12)
	must(os.MkdirAll(dir, 0700), "create config dir")
	must(os.WriteFile(path, []byte(token+"\n"), 0600), "write token")
	fmt.Fprintf(os.Stderr, "Token generated and saved to ~/%s/%s\n", configDir, tokenFile)
	return token
}

func randString(n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	f, _ := os.Open("/dev/urandom")
	defer f.Close()
	raw := make([]byte, n)
	f.Read(raw)
	for i, v := range raw {
		b[i] = chars[int(v)%len(chars)]
	}
	return string(b)
}

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
		fmt.Fprintf(os.Stderr, "Error (%s): %v\n", msg, err)
		os.Exit(1)
	}
}

func die(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
