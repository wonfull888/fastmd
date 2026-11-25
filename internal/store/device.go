package store

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

var ErrPending = errors.New("authorization_pending")
var ErrSlowDown = errors.New("slow_down")

func (s *Store) CreateDevice() (string, string, error) {
	code, err := RandomSecret("")
	if err != nil {
		return "", "", err
	}
	random, err := RandomSecret("")
	if err != nil {
		return "", "", err
	}
	// Only letters and digits, avoiding punctuation in the displayed code.
	random = strings.ToUpper(strings.NewReplacer("-", "A", "_", "B").Replace(random))
	userCode := random[:4] + "-" + random[4:8]
	_, err = s.db.Exec("INSERT INTO device_logins(device_hash,user_code,expires_at) VALUES(?,?,?)", HashToken(code), userCode, time.Now().Unix()+600)
	return code, userCode, err
}
func (s *Store) DecideDevice(userCode, userID string, allow bool) error {
	status := "denied"
	if allow {
		status = "approved"
	}
	result, err := s.db.Exec("UPDATE device_logins SET user_id=?,status=? WHERE user_code=? AND status='pending' AND expires_at>?", userID, status, strings.ToUpper(strings.TrimSpace(userCode)), time.Now().Unix())
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrUnauthorized
	}
	return nil
}
func (s *Store) PollDevice(code string) (string, User, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return "", User{}, err
	}
	defer tx.Rollback()
	var userID sql.NullString
	var status string
	var expiry, last int64
	err = tx.QueryRow("SELECT user_id,status,expires_at,last_poll FROM device_logins WHERE device_hash=?", HashToken(code)).Scan(&userID, &status, &expiry, &last)
	if err != nil || expiry <= time.Now().Unix() || status == "used" || status == "denied" {
		return "", User{}, ErrUnauthorized
	}
	if status == "pending" {
		if time.Now().Unix()-last < 5 {
			return "", User{}, ErrSlowDown
		}
		if _, err := tx.Exec("UPDATE device_logins SET last_poll=? WHERE device_hash=?", time.Now().Unix(), HashToken(code)); err != nil {
			return "", User{}, err
		}
		if err := tx.Commit(); err != nil {
			return "", User{}, err
		}
		return "", User{}, ErrPending
	}
	var u User
	if err := tx.QueryRow("SELECT id,created_at,github_id,github_login FROM users WHERE id=?", userID.String).Scan(&u.ID, &u.CreatedAt, &u.GitHubID, &u.GitHubLogin); err != nil {
		return "", u, err
	}
	token, _, err := issueCredential(tx, u.ID, "cli", "CLI login", "read write delete account", time.Now().Unix()+30*86400, "", "")
	if err != nil {
		return "", u, err
	}
	if _, err := tx.Exec("UPDATE device_logins SET status='used' WHERE device_hash=?", HashToken(code)); err != nil {
		return "", u, err
	}
	if err := tx.Commit(); err != nil {
		return "", u, err
	}
	return token, u, nil
}
