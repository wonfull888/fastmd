package tests

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/wonfull888/fastmd/internal/app"
	"github.com/wonfull888/fastmd/internal/store"
)

func TestCLIBrowserAndTokenLogin(t *testing.T) {
	dir := t.TempDir()
	cli := filepath.Join(dir, "fastmd")
	build := exec.Command("go", "build", "-o", cli, "./cmd/cli")
	build.Dir = ".."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	e := echo.New()
	server := httptest.NewServer(e)
	defer server.Close()
	_, err = app.Register(e, db, app.Options{PublicURL: server.URL, GitHub: app.GitHubConfig{ClientID: "test", ClientSecret: "test"}})
	if err != nil {
		t.Fatal(err)
	}
	user, err := db.GitHubLogin(42, "cli-owner")
	if err != nil {
		t.Fatal(err)
	}
	api, _ := db.DefaultToken(user.ID)
	session, key, err := db.IssueCredential(user.ID, "session", "test", "read write delete account", time.Now().Unix()+60)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(cli, "--server", server.URL, "--json", "login", "--no-browser")
	command.Env = append(os.Environ(), "HOME="+dir, "FASTMD_API_TOKEN=", "FASTMD_BASE_URL="+server.URL)
	var stdout bytes.Buffer
	command.Stdout = &stdout
	stderr, err := command.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if command.ProcessState == nil {
			command.Process.Kill()
			command.Wait()
		}
	})
	verification := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "Open ") {
				verification <- strings.TrimPrefix(line, "Open ")
			}
		}
	}()
	var uri string
	select {
	case uri = <-verification:
	case <-time.After(10 * time.Second):
		t.Fatal("CLI did not print sign-in URL")
	}
	target, _ := url.Parse(uri)
	code := target.Query().Get("user_code")
	if code == "" {
		t.Fatal("verification code missing")
	}
	values := url.Values{"user_code": {code}, "csrf_token": {key.CSRF}, "decision": {"allow"}}
	req, _ := http.NewRequest("POST", server.URL+"/auth/device", strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "fastmd_session", Value: session})
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatal("could not approve CLI")
	}
	if err := command.Wait(); err != nil {
		t.Fatalf("CLI login: %v", err)
	}
	if !strings.Contains(stdout.String(), `"ok":true`) || strings.Contains(stdout.String(), api) {
		t.Fatal("login output invalid or leaked token")
	}
	path := filepath.Join(dir, ".config/fastmd/credentials.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatal("credentials are not protected")
	}
	var saved struct {
		Servers map[string]struct {
			Token string `json:"token"`
		} `json:"servers"`
	}
	json.Unmarshal(data, &saved)
	stored := saved.Servers[server.URL].Token
	if p, err := db.Authenticate(stored); err != nil || p.Kind != "cli" || stored == api {
		t.Fatal("CLI credential incorrect")
	}
	run := func(input string, args ...string) ([]byte, error) {
		cmd := exec.Command(cli, append([]string{"--server", server.URL, "--json"}, args...)...)
		cmd.Env = command.Env
		cmd.Stdin = strings.NewReader(input)
		return cmd.CombinedOutput()
	}
	if out, err := run("", "whoami"); err != nil || !strings.Contains(string(out), "cli-owner") {
		t.Fatalf("whoami: %v %s", err, out)
	}
	if _, err := run("bad-token", "login", "--token-stdin"); err == nil {
		t.Fatal("invalid token accepted")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(after, data) {
		t.Fatal("failed login overwrote credentials")
	}
	if _, err := run("", "logout"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Authenticate(stored); err == nil {
		t.Fatal("CLI credential survived logout")
	}
	if _, err := run(api, "login", "--token-stdin"); err != nil {
		t.Fatal(err)
	}
	if _, err := run("", "logout"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Authenticate(api); err != nil {
		t.Fatal("logout invalidated fixed API token")
	}
}
