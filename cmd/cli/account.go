package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var jsonOutput bool
var requestID string

func parseGlobalArgs() {
	args := []string{os.Args[0]}
	for i := 1; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--json":
			jsonOutput = true
		case "--server", "--request-id":
			name := os.Args[i]
			i++
			if i >= len(os.Args) {
				die("Error: %s requires a value", name)
			}
			if name == "--server" {
				baseURL = strings.TrimRight(os.Args[i], "/")
			} else {
				requestID = os.Args[i]
			}
		default:
			args = append(args, os.Args[i])
		}
	}
	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
		die("Error: server must be an absolute HTTP(S) origin")
	}
	os.Args = args
}

type savedCredential struct {
	Token   string          `json:"token"`
	Account json.RawMessage `json:"account,omitempty"`
}
type credentialFile struct {
	Servers map[string]savedCredential `json:"servers"`
}

func credentialsPath() string {
	home, err := os.UserHomeDir()
	must(err, "get home dir")
	return filepath.Join(home, configDir, "credentials.json")
}
func readCredentials() credentialFile {
	result := credentialFile{Servers: map[string]savedCredential{}}
	data, err := os.ReadFile(credentialsPath())
	if os.IsNotExist(err) {
		return result
	}
	must(err, "read credentials")
	must(json.Unmarshal(data, &result), "parse credentials")
	if result.Servers == nil {
		result.Servers = map[string]savedCredential{}
	}
	return result
}
func saveCredentials(values credentialFile) {
	path := credentialsPath()
	must(os.MkdirAll(filepath.Dir(path), 0700), "create credential directory")
	file, err := os.CreateTemp(filepath.Dir(path), "credentials-*.tmp")
	must(err, "create credential file")
	defer os.Remove(file.Name())
	must(file.Chmod(0600), "protect credentials")
	must(json.NewEncoder(file).Encode(values), "write credentials")
	must(file.Close(), "close credentials")
	must(os.Rename(file.Name(), path), "save credentials")
}
func loadToken() string {
	if token := strings.TrimSpace(os.Getenv("FASTMD_API_TOKEN")); token != "" {
		return token
	}
	values := readCredentials()
	if token := values.Servers[baseURL].Token; token != "" {
		return token
	}

	apiDie("AUTH_REQUIRED", "sign in with fastmd login or configure FASTMD_API_TOKEN; no token was generated")
	return ""
}

func apiDie(code, message string) {
	if jsonOutput {
		json.NewEncoder(os.Stderr).Encode(map[string]any{"ok": false, "error": map[string]string{"code": code, "message": message}})
	} else {
		fmt.Fprintf(os.Stderr, "Error [%s]: %s\n", code, message)
	}
	os.Exit(1)
}
func handleHTTPError(status int, body []byte) {
	var problem struct {
		Code  string `json:"code"`
		Error string `json:"error"`
	}
	json.Unmarshal(body, &problem)
	if problem.Code == "" {
		problem.Code = fmt.Sprintf("HTTP_%d", status)
	}
	if problem.Error == "" {
		problem.Error = http.StatusText(status)
	}
	apiDie(problem.Code, problem.Error)
}
func callAPI(method, path string, body any, authenticated bool) []byte {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		must(err, "encode request")
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, baseURL+path, reader)
	must(err, "create request")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if authenticated {
		req.Header.Set("Authorization", "Bearer "+loadToken())
	}
	response, err := httpClient.Do(req)
	must(err, "request")
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 4*1024*1024))
	must(err, "read response")
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		handleHTTPError(response.StatusCode, data)
	}
	return data
}
func printJSON(data []byte) {
	var value any
	must(json.Unmarshal(data, &value), "parse response")
	must(json.NewEncoder(os.Stdout).Encode(value), "write JSON")
}

func accountCommand(args []string) {
	command := args[0]
	if command == "login" {
		login(args[1:])
		return
	}
	if command == "logout" {
		callAPI("POST", "/v1/auth/logout", map[string]string{}, true)
		values := readCredentials()
		delete(values.Servers, baseURL)
		saveCredentials(values)
		if jsonOutput {
			printJSON([]byte(`{"ok":true}`))
		} else {
			fmt.Println("Signed out of " + baseURL)
		}
		return
	}
	if command == "whoami" {
		data := callAPI("GET", "/v1/auth/me", nil, true)
		printJSON(data)
		return
	}
	if command == "usage" {
		data := callAPI("GET", "/v1/usage", nil, true)
		printJSON(data)
		return
	}
	if command == "list" {
		flags := flag.NewFlagSet("list", flag.ContinueOnError)
		limit := flags.Int("limit", 50, "Page size")
		cursor := flags.String("cursor", "", "Next cursor")
		must(flags.Parse(args[1:]), "list arguments")
		query := url.Values{"limit": {fmt.Sprint(*limit)}}
		if *cursor != "" {
			query.Set("cursor", *cursor)
		}
		printJSON(callAPI("GET", "/v1/docs?"+query.Encode(), nil, true))
		return
	}
	if command == "claim" {
		flags := flag.NewFlagSet("claim", flag.ContinueOnError)
		file := flags.String("token-file", "", "Legacy token file")
		must(flags.Parse(args[1:]), "claim arguments")
		if *file == "" {
			die("Usage: fastmd claim --token-file <path>")
		}
		data, err := os.ReadFile(*file)
		must(err, "read legacy token")
		printJSON(callAPI("POST", "/v1/account/claim", map[string]string{"token": strings.TrimSpace(string(data))}, true))
		return
	}
}

func storeLogin(token string, account json.RawMessage) {
	values := readCredentials()
	values.Servers[baseURL] = savedCredential{Token: token, Account: account}
	saveCredentials(values)
}
func login(args []string) {
	flags := flag.NewFlagSet("login", flag.ContinueOnError)
	tokenStdin := flags.Bool("token-stdin", false, "Read your issued API token from stdin")
	noBrowser := flags.Bool("no-browser", false, "Print the sign-in URL without opening a browser")
	must(flags.Parse(args), "login arguments")
	if flags.NArg() != 0 {
		die("Usage: fastmd login [--no-browser | --token-stdin]")
	}
	if *tokenStdin {
		data, err := io.ReadAll(io.LimitReader(os.Stdin, 4096))
		must(err, "read token")
		token := strings.TrimSpace(string(data))
		if token == "" {
			die("Error: token is empty")
		}
		req, err := http.NewRequest("GET", baseURL+"/v1/auth/me", nil)
		must(err, "create login request")
		req.Header.Set("Authorization", "Bearer "+token)
		response, err := httpClient.Do(req)
		must(err, "validate token")
		defer response.Body.Close()
		body, err := io.ReadAll(io.LimitReader(response.Body, 65536))
		must(err, "read account")
		if response.StatusCode != 200 {
			handleHTTPError(response.StatusCode, body)
		}
		var result struct {
			Account json.RawMessage `json:"account"`
		}
		must(json.Unmarshal(body, &result), "parse account")
		storeLogin(token, result.Account)
		loginResult(result.Account)
		return
	}
	data := callAPI("POST", "/v1/auth/device", map[string]string{}, false)
	var device struct {
		DeviceCode string `json:"device_code"`
		UserCode   string `json:"user_code"`
		URL        string `json:"verification_uri_complete"`
		Interval   int    `json:"interval"`
		Expires    int    `json:"expires_in"`
	}
	must(json.Unmarshal(data, &device), "parse login request")
	if device.DeviceCode == "" || device.URL == "" || device.Interval < 1 || device.Expires < 1 {
		die("Error: invalid login response")
	}
	target, err := url.Parse(device.URL)
	must(err, "parse login URL")
	if target.Scheme+"://"+target.Host != baseURL {
		die("Error: login URL does not match the selected server")
	}
	fmt.Fprintf(os.Stderr, "Open %s\nConfirm this code: %s\n", device.URL, device.UserCode)
	if !*noBrowser {
		var command *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			command = exec.Command("open", device.URL)
		case "windows":
			command = exec.Command("rundll32", "url.dll,FileProtocolHandler", device.URL)
		default:
			command = exec.Command("xdg-open", device.URL)
		}
		if err := command.Run(); err != nil {
			fmt.Fprintln(os.Stderr, "Open the link above in your browser to continue.")
		}
	}
	deadline := time.Now().Add(time.Duration(device.Expires) * time.Second)
	interval := time.Duration(device.Interval) * time.Second
	for time.Now().Before(deadline) {
		time.Sleep(interval)
		encoded, _ := json.Marshal(map[string]string{"device_code": device.DeviceCode})
		response, err := httpClient.Post(baseURL+"/v1/auth/device/token", "application/json", bytes.NewReader(encoded))
		must(err, "check login")
		body, err := io.ReadAll(io.LimitReader(response.Body, 65536))
		response.Body.Close()
		must(err, "read login")
		if response.StatusCode == 200 {
			var result struct {
				Token   string          `json:"token"`
				Account json.RawMessage `json:"account"`
			}
			must(json.Unmarshal(body, &result), "parse login")
			if result.Token == "" {
				die("Error: server did not issue a CLI credential")
			}
			storeLogin(result.Token, result.Account)
			// A legacy local token proves ownership; never replace it or print it.
			if baseURL == "https://fastmd.dev" {
				home, err := os.UserHomeDir()
				must(err, "get home dir")
				if legacy, err := os.ReadFile(filepath.Join(home, configDir, tokenFile)); err == nil && strings.TrimSpace(string(legacy)) != "" {
					callAPI("POST", "/v1/account/claim", map[string]string{"token": strings.TrimSpace(string(legacy))}, true)
				}
			}
			loginResult(result.Account)
			return
		}
		var problem struct {
			Error string `json:"error"`
		}
		json.Unmarshal(body, &problem)
		if problem.Error == "authorization_pending" {
			continue
		}
		if problem.Error == "slow_down" {
			interval += 5 * time.Second
			continue
		}
		apiDie("LOGIN_FAILED", "login expired or was cancelled; run fastmd login again")
	}
	apiDie("LOGIN_EXPIRED", "run fastmd login again")
}
func loginResult(account json.RawMessage) {
	if jsonOutput {
		encoded, _ := json.Marshal(map[string]any{"ok": true, "account": account, "server": baseURL})
		printJSON(encoded)
	} else {
		fmt.Println("Signed in to " + baseURL)
	}
}
