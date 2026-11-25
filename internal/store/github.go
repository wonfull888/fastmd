package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

func migrateGitHub(db *sql.DB) error {
	for name, definition := range map[string]string{"github_id": "INTEGER", "github_login": "TEXT NOT NULL DEFAULT ''"} {
		rows, err := db.Query("PRAGMA table_info(users)")
		if err != nil {
			return err
		}
		found := false
		for rows.Next() {
			var cid, required, primary int
			var n, kind string
			var defaultValue sql.NullString
			if err := rows.Scan(&cid, &n, &kind, &required, &defaultValue, &primary); err != nil {
				rows.Close()
				return err
			}
			if n == name {
				found = true
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if !found {
			if _, err := db.Exec("ALTER TABLE users ADD COLUMN " + name + " " + definition); err != nil {
				return err
			}
		}
	}
	_, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_github ON users(github_id);
 CREATE TABLE IF NOT EXISTS account_tokens (user_id TEXT PRIMARY KEY REFERENCES users(id), credential_id TEXT NOT NULL UNIQUE REFERENCES credentials(id), encrypted BLOB NOT NULL);
 CREATE UNIQUE INDEX IF NOT EXISTS idx_one_api_token ON credentials(user_id) WHERE kind='api' AND revoked_at=0;
 CREATE TABLE IF NOT EXISTS github_logins (state_hash TEXT PRIMARY KEY, verifier TEXT NOT NULL, return_path TEXT NOT NULL, expires_at INTEGER NOT NULL);
 CREATE TABLE IF NOT EXISTS device_logins (device_hash TEXT PRIMARY KEY, user_code TEXT NOT NULL UNIQUE, user_id TEXT, expires_at INTEGER NOT NULL, status TEXT NOT NULL DEFAULT 'pending', last_poll INTEGER NOT NULL DEFAULT 0);`)
	return err
}

// The independent key must be backed up with the database. Never derive it from
// an OAuth client secret: rotating provider credentials must not change tokens.
func loadTokenKey(db *sql.DB, path string) ([]byte, error) {
	key := make([]byte, 32)
	if path == ":memory:" {
		_, err := rand.Read(key)
		return key, err
	}
	keyPath := path + ".key"
	data, err := os.ReadFile(keyPath)
	if err == nil {
		if len(data) != 32 {
			return nil, errors.New("invalid database encryption key")
		}
		return data, nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM account_tokens").Scan(&count); err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("database encryption key is missing; restore the .key backup")
	}
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(err) {
		return loadTokenKey(db, path)
	}
	if err != nil {
		return nil, err
	}
	_, err = file.Write(key)
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	return key, err
}
func (s *Store) tokenCipher() (cipher.AEAD, error) {
	block, err := aes.NewCipher(s.tokenKey)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func (s *Store) GitHubLogin(id int64, login string) (User, error) {
	if id <= 0 || login == "" {
		return User{}, errors.New("invalid GitHub identity")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()
	uid, err := RandomSecret("user_")
	if err != nil {
		return User{}, err
	}
	_, err = tx.Exec("INSERT INTO users(id,email,created_at,github_id,github_login) VALUES(?,?,?,?,?) ON CONFLICT(github_id) DO UPDATE SET github_login=excluded.github_login", uid, "github:"+strconv.FormatInt(id, 10), time.Now().Unix(), id, login)
	if err != nil {
		return User{}, err
	}
	var u User
	if err := tx.QueryRow("SELECT id,created_at,github_id,github_login FROM users WHERE github_id=?", id).Scan(&u.ID, &u.CreatedAt, &u.GitHubID, &u.GitHubLogin); err != nil {
		return User{}, err
	}
	var count int
	if err := tx.QueryRow("SELECT COUNT(*) FROM account_tokens WHERE user_id=?", u.ID).Scan(&count); err != nil {
		return User{}, err
	}
	if count == 0 {
		raw, c, err := issueCredential(tx, u.ID, "api", "Default API token", "read write delete", 0, "", "")
		if err != nil {
			return User{}, err
		}
		aead, err := s.tokenCipher()
		if err != nil {
			return User{}, err
		}
		nonce := make([]byte, aead.NonceSize())
		if _, err := rand.Read(nonce); err != nil {
			return User{}, err
		}
		encrypted := aead.Seal(nonce, nonce, []byte(raw), []byte(u.ID))
		if _, err := tx.Exec("INSERT INTO account_tokens(user_id,credential_id,encrypted) VALUES(?,?,?)", u.ID, c.ID, encrypted); err != nil {
			return User{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return User{}, err
	}
	return u, nil
}
func (s *Store) DefaultToken(userID string) (string, error) {
	var encrypted []byte
	if err := s.db.QueryRow("SELECT encrypted FROM account_tokens WHERE user_id=?", userID).Scan(&encrypted); err != nil {
		return "", err
	}
	aead, err := s.tokenCipher()
	if err != nil {
		return "", err
	}
	if len(encrypted) < aead.NonceSize() {
		return "", fmt.Errorf("invalid encrypted token")
	}
	raw, err := aead.Open(nil, encrypted[:aead.NonceSize()], encrypted[aead.NonceSize():], []byte(userID))
	return string(raw), err
}
func (s *Store) BeginGitHub(state, verifier, path string) error {
	_, err := s.db.Exec("INSERT INTO github_logins(state_hash,verifier,return_path,expires_at) VALUES(?,?,?,?)", HashToken(state), verifier, path, time.Now().Unix()+600)
	return err
}
func (s *Store) ConsumeGitHub(state string) (string, string, error) {
	var verifier, path string
	err := s.db.QueryRow("DELETE FROM github_logins WHERE state_hash=? AND expires_at>? RETURNING verifier,return_path", HashToken(state), time.Now().Unix()).Scan(&verifier, &path)
	return verifier, path, err
}
