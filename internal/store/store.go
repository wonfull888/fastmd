package store

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

const alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// Document represents a stored Markdown document.
type Document struct {
	ID        string
	Content   string
	TokenHash string
	CreatedAt int64
	ExpiresAt *int64
}

// Store wraps the SQLite database.
type Store struct {
	db *sql.DB
}

// New opens (or creates) the SQLite database at path.
func New(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_journal=WAL&_timeout=5000&_foreign_keys=on")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite: serialise writes
	if err := migrate(db); err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS documents (
			id          TEXT PRIMARY KEY,
			content     TEXT    NOT NULL,
			token_hash  TEXT    NOT NULL,
			ip_hash     TEXT,
			created_at  INTEGER NOT NULL,
			expires_at  INTEGER DEFAULT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_token_hash ON documents(token_hash);
		CREATE INDEX IF NOT EXISTS idx_expires_at  ON documents(expires_at);
	`)
	return err
}

// HashToken returns SHA-256 hex of a token string.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", sum)
}

// GenerateID returns a unique Base62 ID of the given length.
// It retries on collision (extremely rare with 4+ chars).
func (s *Store) GenerateID(length int) (string, error) {
	for i := 0; i < 10; i++ {
		b := make([]byte, length)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		for j, v := range b {
			b[j] = alphabet[int(v)%len(alphabet)]
		}
		id := string(b)
		// Check uniqueness
		var exists int
		_ = s.db.QueryRow(`SELECT COUNT(*) FROM documents WHERE id = ?`, id).Scan(&exists)
		if exists == 0 {
			return id, nil
		}
	}
	return "", fmt.Errorf("failed to generate unique ID after 10 attempts")
}

// Create inserts a new document and returns its ID.
func (s *Store) Create(content, tokenHash, ipHash string) (string, error) {
	id, err := s.GenerateID(4)
	if err != nil {
		return "", err
	}
	_, err = s.db.Exec(
		`INSERT INTO documents (id, content, token_hash, ip_hash, created_at) VALUES (?, ?, ?, ?, ?)`,
		id, content, tokenHash, ipHash, time.Now().Unix(),
	)
	return id, err
}

// GetByID returns a document by ID, or nil if not found / expired.
func (s *Store) GetByID(id string) (*Document, error) {
	row := s.db.QueryRow(
		`SELECT id, content, token_hash, created_at, expires_at
		 FROM documents
		 WHERE id = ? AND (expires_at IS NULL OR expires_at > ?)`,
		id, time.Now().Unix(),
	)
	var doc Document
	err := row.Scan(&doc.ID, &doc.Content, &doc.TokenHash, &doc.CreatedAt, &doc.ExpiresAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

// Delete removes a document if the tokenHash matches.
// Returns true if a row was deleted, false if not found or token mismatch.
func (s *Store) Delete(id, tokenHash string) (bool, error) {
	res, err := s.db.Exec(
		`DELETE FROM documents WHERE id = ? AND token_hash = ?`,
		id, tokenHash,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
