package store

import (
	"database/sql"
	"errors"
	"time"
)

func (s *Store) CreateOwned(userID, content, requestID string) (string, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if requestID != "" {
		var oldHash, id string
		err := tx.QueryRow("SELECT content_hash,document_id FROM create_requests WHERE user_id=? AND request_id=?", userID, requestID).Scan(&oldHash, &id)
		if err == nil {
			if oldHash != HashToken(content) {
				return "", ErrConflict
			}
			var count int
			if err := tx.QueryRow("SELECT COUNT(*) FROM documents WHERE id=? AND user_id=? AND (expires_at IS NULL OR expires_at>?)", id, userID, time.Now().Unix()).Scan(&count); err != nil {
				return "", err
			}
			if count == 0 {
				return "", ErrGone
			}
			return id, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
	}
	id := ""
	for attempt := 0; attempt < 10; attempt++ {
		random, err := RandomSecret("")
		if err != nil {
			return "", err
		}
		// Keep links compatible with the existing Base62 ID format.
		candidate := ""
		for _, c := range random {
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
				candidate += string(c)
				if len(candidate) == 8 {
					break
				}
			}
		}
		if len(candidate) != 8 {
			continue
		}
		result, err := tx.Exec("INSERT OR IGNORE INTO documents(id,content,token_hash,user_id,created_at) VALUES(?,?,?,?,?)", candidate, content, HashToken("user:"+userID), userID, time.Now().Unix())
		if err != nil {
			return "", err
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return "", err
		}
		if rows == 1 {
			id = candidate
			break
		}
	}
	if id == "" {
		return "", errors.New("failed to allocate document ID")
	}
	if requestID != "" {
		if _, err := tx.Exec("INSERT INTO create_requests(user_id,request_id,content_hash,document_id) VALUES(?,?,?,?)", userID, requestID, HashToken(content), id); err != nil {
			return "", err
		}
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) UpdateOwned(userID, id, content string) (bool, error) {
	result, err := s.db.Exec("UPDATE documents SET content=? WHERE id=? AND user_id=? AND (expires_at IS NULL OR expires_at>?)", content, id, userID, time.Now().Unix())
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n > 0, err
}

func (s *Store) DeleteOwned(userID, id string) (bool, error) {
	result, err := s.db.Exec("DELETE FROM documents WHERE id=? AND user_id=?", id, userID)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n > 0, err
}

func (s *Store) ListOwned(userID string, offset, limit int) ([]Document, error) {
	rows, err := s.db.Query("SELECT id,content,created_at FROM documents WHERE user_id=? AND (expires_at IS NULL OR expires_at>?) ORDER BY created_at DESC,id DESC LIMIT ? OFFSET ?", userID, time.Now().Unix(), limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Document{}
	for rows.Next() {
		var d Document
		if err := rows.Scan(&d.ID, &d.Content, &d.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	return items, rows.Err()
}

func (s *Store) DocumentCount(userID string) (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM documents WHERE user_id=? AND (expires_at IS NULL OR expires_at>?)", userID, time.Now().Unix()).Scan(&count)
	return count, err
}
