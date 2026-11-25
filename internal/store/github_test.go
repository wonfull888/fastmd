package store

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestFixedTokenIdentityAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "accounts.db")
	s, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	user, err := s.GitHubLogin(42, "original")
	if err != nil {
		t.Fatal(err)
	}
	token, err := s.DefaultToken(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	var encrypted []byte
	if err := s.db.QueryRow("SELECT encrypted FROM account_tokens WHERE user_id=?", user.ID).Scan(&encrypted); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encrypted, []byte(token)) {
		t.Fatal("token stored in plaintext")
	}
	var group sync.WaitGroup
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			u, err := s.GitHubLogin(42, "renamed")
			if err != nil {
				t.Error(err)
				return
			}
			if u.ID != user.ID {
				t.Error("rename created a new account")
			}
		}()
	}
	group.Wait()
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM credentials WHERE user_id=? AND kind='api'", user.ID).Scan(&count)
	if count != 1 {
		t.Fatalf("expected one API token, got %d", count)
	}
	session, credential, err := s.IssueCredential(user.ID, "session", "test", "read write delete account", time.Now().Unix()+60)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(session); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RevokeCredential(user.ID, credential.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(session); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("session remained active")
	}
	api, err := s.Authenticate(token)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := s.RevokeCredential(user.ID, api.ID); err != nil || ok {
		t.Fatal("fixed API token could be revoked")
	}
	if _, _, err := s.IssueCredential(user.ID, "api", "extra", "read", 0); err == nil {
		t.Fatal("manual API token was issued")
	}
	s.Close()
	s, err = New(path)
	if err != nil {
		t.Fatal(err)
	}
	same, err := s.DefaultToken(user.ID)
	if err != nil || same != token {
		t.Fatal("token changed after restart")
	}
	if p, err := s.Authenticate(token); err != nil || p.User.GitHubLogin != "renamed" {
		t.Fatal("identity did not persist")
	}
	s.Close()
	if err := os.Remove(path + ".key"); err != nil {
		t.Fatal(err)
	}
	if reopened, err := New(path); err == nil {
		reopened.Close()
		t.Fatal("missing encryption key silently replaced")
	}
}

func TestDeviceLoginAndLegacyClaim(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	user, err := s.GitHubLogin(42, "owner")
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.GitHubLogin(43, "other")
	if err != nil {
		t.Fatal(err)
	}
	token, _ := s.DefaultToken(user.ID)
	code, userCode, err := s.CreateDevice()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.PollDevice(code); !errors.Is(err, ErrPending) {
		t.Fatalf("want pending: %v", err)
	}
	if _, _, err := s.PollDevice(code); !errors.Is(err, ErrSlowDown) {
		t.Fatalf("want slow_down: %v", err)
	}
	if err := s.DecideDevice(userCode, user.ID, true); err != nil {
		t.Fatal(err)
	}
	cli, u, err := s.PollDevice(code)
	if err != nil || u.ID != user.ID || cli == token {
		t.Fatal("CLI authorization failed")
	}
	if _, _, err := s.PollDevice(code); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("device code replay accepted")
	}
	if p, err := s.Authenticate(cli); err != nil || p.Kind != "cli" {
		t.Fatal("CLI credential invalid")
	}
	id, err := s.Create("# Old document", HashToken("legacy"), "")
	if err != nil {
		t.Fatal(err)
	}
	if n, err := s.ClaimLegacy(user.ID, "wrong"); err != nil || n != 0 {
		t.Fatal("wrong legacy token claimed document")
	}
	if n, err := s.ClaimLegacy(user.ID, "legacy"); err != nil || n != 1 {
		t.Fatal("legacy ownership not migrated")
	}
	if n, err := s.ClaimLegacy(other.ID, "legacy"); err != nil || n != 0 {
		t.Fatal("legacy doc stolen after migration")
	}
	doc, _ := s.GetByID(id)
	if doc.ID != id || doc.UserID != user.ID || doc.Content != "# Old document" {
		t.Fatal("legacy document damaged")
	}
}
