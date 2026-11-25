package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wonfull888/fastmd/internal/store"
)

// Exercise the actual CLI, shell helper, installer and server with an isolated
// database and HOME. These requests never touch fastmd.dev or the user's token.
func TestCLIAndSkillEndToEnd(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	cli := filepath.Join(dir, "fastmd")
	server := filepath.Join(dir, "fastmd-server")
	for _, target := range []struct{ output, pkg string }{{cli, "./cmd/cli"}, {server, "./cmd/server"}} {
		cmd := exec.Command("go", "build", "-o", target.output, target.pkg)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v\n%s", target.pkg, err, out)
		}
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	endpoint := fmt.Sprintf("http://127.0.0.1:%d", port)
	log, err := os.Create(filepath.Join(dir, "server.log"))
	if err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(dir, "test.db")
	db, err := store.New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	user, err := db.GitHubLogin(1, "e2e-owner")
	if err != nil {
		t.Fatal(err)
	}
	token, err := db.DefaultToken(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	cmd := exec.Command(server, "--port", fmt.Sprint(port), "--db", dbPath)
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait(); log.Close() })
	client := &http.Client{Timeout: 2 * time.Second}
	ready := false
	for i := 0; i < 100; i++ {
		response, err := client.Get(endpoint + "/v1/version")
		if err == nil {
			response.Body.Close()
			if response.StatusCode == 200 {
				ready = true
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ready {
		data, _ := os.ReadFile(filepath.Join(dir, "server.log"))
		t.Fatalf("server did not start: %s", data)
	}
	home := filepath.Join(dir, "home")
	if err := os.MkdirAll(home, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".config/fastmd"), 0700); err != nil {
		t.Fatal(err)
	}
	credential, _ := json.Marshal(map[string]any{"servers": map[string]any{endpoint: map[string]string{"token": token}}})
	if err := os.WriteFile(filepath.Join(home, ".config/fastmd/credentials.json"), credential, 0600); err != nil {
		t.Fatal(err)
	}
	envFor := func(home string) []string {
		env := []string{}
		for _, item := range os.Environ() {
			if !strings.HasPrefix(item, "HOME=") && !strings.HasPrefix(item, "FASTMD_BASE_URL=") && !strings.HasPrefix(item, "FASTMD_API_TOKEN=") {
				env = append(env, item)
			}
		}
		return append(env, "HOME="+home, "FASTMD_BASE_URL="+endpoint)
	}
	run := func(home, input string, wantOK bool, executable string, args ...string) string {
		t.Helper()
		command := exec.Command(executable, args...)
		command.Env, command.Dir = envFor(home), dir
		command.Stdin = strings.NewReader(input)
		var stdout, stderr bytes.Buffer
		command.Stdout, command.Stderr = &stdout, &stderr
		err := command.Run()
		if (err == nil) != wantOK {
			t.Fatalf("%s %v: %v\nstdout: %s\nstderr: %s", executable, args, err, &stdout, &stderr)
		}
		return stdout.String()
	}
	write := func(name, content string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	parseURL := func(output string) (string, string) {
		t.Helper()
		fields := strings.Fields(output)
		if len(fields) == 0 {
			t.Fatal("missing document URL")
		}
		link := fields[len(fields)-1]
		parsed, err := url.Parse(link)
		if err != nil || !strings.HasPrefix(link, endpoint+"/") {
			t.Fatalf("unexpected URL: %q", output)
		}
		return strings.TrimSuffix(strings.TrimPrefix(parsed.Path, "/"), ".md"), link
	}
	raw := func(id, expected string, status int) {
		t.Helper()
		req, _ := http.NewRequest("GET", endpoint+"/"+id+".md", nil)
		req.Header.Set("Accept", "text/plain")
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		body, _ := io.ReadAll(response.Body)
		if response.StatusCode != status || (status == 200 && string(body) != expected) {
			t.Fatalf("raw read: status %d, body %q, want %d, %q", response.StatusCode, body, status, expected)
		}
	}
	original := "# Original\n\nOriginal document.\n"
	changed := "# Updated\n\n中文 😀 \"quoted\" \\path\tend\r\n\n```sh\nprintf '\\n'\n```\n\n"
	originalFile := write("original.md", original)
	changedFile := write("changed.md", changed)
	emptyFile := write("empty.md", " \n\t")
	id, link := parseURL(run(home, "", true, cli, "push", originalFile))
	raw(id, original, 200)
	updatedID, updatedLink := parseURL(run(home, "", true, cli, "update", id, changedFile))
	if updatedID != id || updatedLink != link {
		t.Fatal("CLI update changed the URL")
	}
	raw(id, changed, 200)
	run(home, "", true, cli, "get", id)
	downloaded, err := os.ReadFile(filepath.Join(dir, "updated.md"))
	if err != nil || string(downloaded) != changed {
		t.Fatalf("CLI get damaged Markdown: %q, %v", downloaded, err)
	}
	run(home, "", false, cli, "update", id, emptyFile)
	run(home, "", false, cli, "update", id, filepath.Join(dir, "missing.md"))
	run(home, "", false, cli, "update", id)
	run(home, "", false, cli, "update", link, changedFile)
	noToken := filepath.Join(dir, "no-token")
	run(noToken, "", false, cli, "update", id, changedFile)
	if _, err := os.Stat(filepath.Join(noToken, ".config/fastmd/token")); !os.IsNotExist(err) {
		t.Fatal("update generated a new token")
	}
	raw(id, changed, 200)
	// Install the skill into all three isolated client directories.
	run(home, "", true, "sh", filepath.Join(root, "install-skill.sh"), "--all")
	for _, location := range []string{".claude/skills/fastmd", ".config/opencode/skills/fastmd", ".codex/skills/fastmd"} {
		if _, err := os.Stat(filepath.Join(home, location, "SKILL.md")); err != nil {
			t.Fatal(err)
		}
	}
	helper := filepath.Join(home, ".codex/skills/fastmd/scripts/publish.sh")
	_, skillLink := parseURL(run(home, original, true, "sh", helper, "--update", id))
	if skillLink != link {
		t.Fatal("skill update changed the URL")
	}
	raw(id, original, 200)
	run(home, "", true, "sh", helper, "--update", id, "--file", changedFile)
	raw(id, changed, 200)
	run(noToken, "", false, "sh", helper, "--update", id, "--file", changedFile)
	run(home, "", false, "sh", helper, "--update", id, "--file", emptyFile)
	run(home, "", false, "sh", helper, "--update", "notFound", "--file", changedFile)
	run(home, "", false, "sh", helper, "--update", "../unsafe", "--file", changedFile)
	otherHome := filepath.Join(dir, "other-home")
	if err := os.MkdirAll(filepath.Join(otherHome, ".config/fastmd"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(otherHome, ".config/fastmd/token"), []byte("other-token\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run(otherHome, "", false, cli, "update", id, changedFile)
	run(otherHome, "", false, "sh", helper, "--update", id, "--file", changedFile)
	raw(id, changed, 200)
	// New publishing still works through both stdin entrypoints, with exact bytes.
	pipeID, _ := parseURL(run(home, changed, true, cli))
	raw(pipeID, changed, 200)
	skillID, _ := parseURL(run(home, changed, true, "sh", helper))
	raw(skillID, changed, 200)
	freshHome := filepath.Join(dir, "fresh-skill-home")
	run(freshHome, original, false, "sh", helper)
	// List ownership and ensure updates have not created extra documents.
	req, _ := http.NewRequest("GET", endpoint+"/v1/docs", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var listed struct{ Documents []struct{ ID string } }
	err = json.NewDecoder(response.Body).Decode(&listed)
	response.Body.Close()
	if err != nil || len(listed.Documents) != 3 {
		t.Fatalf("list expected 3 documents, got %+v, %v", listed, err)
	}
	run(otherHome, "", false, cli, "delete", id)
	raw(id, changed, 200)
	run(home, "", true, cli, "delete", id)
	raw(id, "", 404)
	run(home, "", false, cli, "update", id, changedFile)
	run(home, "", false, "sh", helper, "--update", id, "--file", changedFile)
	raw(id, "", 404)
	t.Log("CLI push/pipe/get/update/delete, skill publish/update/installer, content integrity and failure paths passed")
}
