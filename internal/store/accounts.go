package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"strings"
	"time"
)

var (
	ErrUnauthorized = errors.New("invalid or expired credential")
	ErrConflict     = errors.New("request ID was already used for different content")
	ErrGone         = errors.New("the original request's document is no longer available")
)

type User struct {
	ID          string `json:"id"`
	Email       string `json:"-"`
	GitHubID    int64  `json:"github_id"`
	GitHubLogin string `json:"github_login"`
	CreatedAt   int64  `json:"created_at"`
}

type Credential struct {
	ID         string `json:"id"`
	User       User   `json:"-"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Scopes     string `json:"scopes"`
	CreatedAt  int64  `json:"created_at"`
	ExpiresAt  int64  `json:"expires_at"`
	LastUsedAt int64  `json:"last_used_at"`
	RevokedAt  int64  `json:"revoked_at"`
	CSRF       string `json:"-"`
	GrantID    string `json:"-"`
	ClientID   string `json:"-"`
}

func (c Credential) Allows(scope string) bool {
	for _, allowed := range strings.Fields(c.Scopes) {
		if allowed == scope {
			return true
		}
	}
	return false
}

func RandomSecret(prefix string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefix + base64.RawURLEncoding.EncodeToString(b), nil
}

func migrateAccounts(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS users (id TEXT PRIMARY KEY, email TEXT NOT NULL UNIQUE, created_at INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS credentials (id TEXT PRIMARY KEY, token_hash TEXT NOT NULL UNIQUE, user_id TEXT NOT NULL REFERENCES users(id), kind TEXT NOT NULL, name TEXT NOT NULL, scopes TEXT NOT NULL, created_at INTEGER NOT NULL, expires_at INTEGER NOT NULL DEFAULT 0, last_used_at INTEGER NOT NULL DEFAULT 0, revoked_at INTEGER NOT NULL DEFAULT 0, csrf TEXT NOT NULL DEFAULT '', grant_id TEXT NOT NULL DEFAULT '', client_id TEXT NOT NULL DEFAULT '');
CREATE INDEX IF NOT EXISTS idx_credential_user ON credentials(user_id);
CREATE TABLE IF NOT EXISTS create_requests (user_id TEXT NOT NULL, request_id TEXT NOT NULL, content_hash TEXT NOT NULL, document_id TEXT NOT NULL, PRIMARY KEY(user_id, request_id));
CREATE TABLE IF NOT EXISTS oauth_clients (id TEXT PRIMARY KEY, name TEXT NOT NULL, redirect_uris TEXT NOT NULL, created_at INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS oauth_codes (token_hash TEXT PRIMARY KEY, user_id TEXT NOT NULL, client_id TEXT NOT NULL, redirect_uri TEXT NOT NULL, challenge TEXT NOT NULL, resource TEXT NOT NULL, scopes TEXT NOT NULL, expires_at INTEGER NOT NULL, used INTEGER NOT NULL DEFAULT 0);
CREATE TABLE IF NOT EXISTS oauth_grants (id TEXT PRIMARY KEY, user_id TEXT NOT NULL, client_id TEXT NOT NULL, name TEXT NOT NULL, scopes TEXT NOT NULL, created_at INTEGER NOT NULL, revoked_at INTEGER NOT NULL DEFAULT 0);
`)
	if err != nil {
		return err
	}
	rows, err := db.Query("PRAGMA table_info(documents)")
	if err != nil {
		return err
	}
	hasOwner := false
	for rows.Next() {
		var cid, required, primary int
		var name, kind string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &kind, &required, &defaultValue, &primary); err != nil {
			rows.Close()
			return err
		}
		if name == "user_id" {
			hasOwner = true
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if !hasOwner {
		if _, err := db.Exec("ALTER TABLE documents ADD COLUMN user_id TEXT REFERENCES users(id)"); err != nil {
			return err
		}
	}
	_, err = db.Exec("CREATE INDEX IF NOT EXISTS idx_documents_user ON documents(user_id,created_at)")
	if err != nil {
		return err
	}
	return migrateGitHub(db)
}

type executor interface {
	Exec(string, ...any) (sql.Result, error)
}

func issueCredential(db executor, userID, kind, name, scopes string, expiry int64, grantID, clientID string) (string, Credential, error) {
	raw, err := RandomSecret("fmd_live_")
	if err != nil {
		return "", Credential{}, err
	}
	id, err := RandomSecret("key_")
	if err != nil {
		return "", Credential{}, err
	}
	csrf := ""
	if kind == "session" {
		csrf, err = RandomSecret("csrf_")
		if err != nil {
			return "", Credential{}, err
		}
	}
	c := Credential{ID: id, Kind: kind, Name: name, Scopes: scopes, CreatedAt: time.Now().Unix(), ExpiresAt: expiry, CSRF: csrf, GrantID: grantID, ClientID: clientID}
	_, err = db.Exec("INSERT INTO credentials(id,token_hash,user_id,kind,name,scopes,created_at,expires_at,csrf,grant_id,client_id) VALUES(?,?,?,?,?,?,?,?,?,?,?)", id, HashToken(raw), userID, kind, name, scopes, c.CreatedAt, expiry, csrf, grantID, clientID)
	return raw, c, err
}

func (s *Store) IssueCredential(userID, kind, name, scopes string, expiry int64) (string, Credential, error) {
	if kind == "api" {
		return "", Credential{}, errors.New("API tokens are assigned automatically")
	}
	return issueCredential(s.db, userID, kind, name, scopes, expiry, "", "")
}

func (s *Store) Authenticate(raw string) (Credential, error) {
	var c Credential
	err := s.db.QueryRow(`SELECT c.id,c.kind,c.name,c.scopes,c.created_at,c.expires_at,c.last_used_at,c.revoked_at,c.csrf,c.grant_id,c.client_id,u.id,u.email,u.created_at,COALESCE(u.github_id,0),u.github_login
FROM credentials c JOIN users u ON u.id=c.user_id
WHERE c.token_hash=? AND c.kind<>'refresh' AND c.revoked_at=0 AND (c.expires_at=0 OR c.expires_at>?)
AND (c.grant_id='' OR EXISTS(SELECT 1 FROM oauth_grants g WHERE g.id=c.grant_id AND g.revoked_at=0))`, HashToken(raw), time.Now().Unix()).Scan(&c.ID, &c.Kind, &c.Name, &c.Scopes, &c.CreatedAt, &c.ExpiresAt, &c.LastUsedAt, &c.RevokedAt, &c.CSRF, &c.GrantID, &c.ClientID, &c.User.ID, &c.User.Email, &c.User.CreatedAt, &c.User.GitHubID, &c.User.GitHubLogin)
	if errors.Is(err, sql.ErrNoRows) {
		return c, ErrUnauthorized
	}
	if err != nil {
		return c, err
	}
	_, err = s.db.Exec("UPDATE credentials SET last_used_at=? WHERE id=?", time.Now().Unix(), c.ID)
	return c, err
}

func (s *Store) RevokeCredential(userID, id string) (bool, error) {
	result, err := s.db.Exec("UPDATE credentials SET revoked_at=? WHERE id=? AND user_id=? AND kind<>'api' AND revoked_at=0", time.Now().Unix(), id, userID)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n > 0, err
}

func (s *Store) ClaimLegacy(userID, token string) (int64, error) {
	result, err := s.db.Exec("UPDATE documents SET user_id=? WHERE user_id IS NULL AND token_hash=?", userID, HashToken(token))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
