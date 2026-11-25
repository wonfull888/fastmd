package store

import (
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type OAuthClient struct {
	ID           string   `json:"client_id"`
	Name         string   `json:"client_name"`
	RedirectURIs []string `json:"redirect_uris"`
}

type OAuthGrant struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Scopes    string `json:"scopes"`
	CreatedAt int64  `json:"created_at"`
	RevokedAt int64  `json:"revoked_at"`
}

type OAuthTokens struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

func (s *Store) RegisterClient(name string, redirects []string) (OAuthClient, error) {
	id, err := RandomSecret("client_")
	if err != nil {
		return OAuthClient{}, err
	}
	encoded, err := json.Marshal(redirects)
	if err != nil {
		return OAuthClient{}, err
	}
	_, err = s.db.Exec("INSERT INTO oauth_clients(id,name,redirect_uris,created_at) VALUES(?,?,?,?)", id, name, string(encoded), time.Now().Unix())
	return OAuthClient{ID: id, Name: name, RedirectURIs: redirects}, err
}

func (s *Store) OAuthClient(id string) (OAuthClient, error) {
	var c OAuthClient
	var redirects string
	err := s.db.QueryRow("SELECT id,name,redirect_uris FROM oauth_clients WHERE id=?", id).Scan(&c.ID, &c.Name, &redirects)
	if err != nil {
		return c, err
	}
	err = json.Unmarshal([]byte(redirects), &c.RedirectURIs)
	return c, err
}

func (s *Store) AuthorizationCode(userID, clientID, redirect, challenge, resource, scopes string) (string, error) {
	code, err := RandomSecret("code_")
	if err != nil {
		return "", err
	}
	_, err = s.db.Exec("INSERT INTO oauth_codes(token_hash,user_id,client_id,redirect_uri,challenge,resource,scopes,expires_at) VALUES(?,?,?,?,?,?,?,?)", HashToken(code), userID, clientID, redirect, challenge, resource, scopes, time.Now().Unix()+120)
	return code, err
}

func issueOAuthTokens(tx *sql.Tx, userID, clientID, grantID, scopes string) (OAuthTokens, error) {
	access, _, err := issueCredential(tx, userID, "oauth", "MCP access", scopes, time.Now().Unix()+3600, grantID, clientID)
	if err != nil {
		return OAuthTokens{}, err
	}
	refresh, _, err := issueCredential(tx, userID, "refresh", "MCP refresh", scopes, time.Now().Unix()+30*86400, grantID, clientID)
	if err != nil {
		return OAuthTokens{}, err
	}
	return OAuthTokens{AccessToken: access, TokenType: "Bearer", ExpiresIn: 3600, RefreshToken: refresh, Scope: scopes}, nil
}

func (s *Store) ExchangeCode(code, clientID, redirect, challenge, resource string) (OAuthTokens, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return OAuthTokens{}, err
	}
	defer tx.Rollback()
	var userID, registeredClient, registeredRedirect, expected, expectedResource, scopes string
	var expires int64
	var used int
	err = tx.QueryRow("SELECT user_id,client_id,redirect_uri,challenge,resource,scopes,expires_at,used FROM oauth_codes WHERE token_hash=?", HashToken(code)).Scan(&userID, &registeredClient, &registeredRedirect, &expected, &expectedResource, &scopes, &expires, &used)
	if errors.Is(err, sql.ErrNoRows) {
		return OAuthTokens{}, ErrUnauthorized
	}
	if err != nil {
		return OAuthTokens{}, err
	}
	if used != 0 || expires <= time.Now().Unix() || registeredClient != clientID || registeredRedirect != redirect || expectedResource != resource || subtle.ConstantTimeCompare([]byte(expected), []byte(challenge)) != 1 {
		return OAuthTokens{}, ErrUnauthorized
	}
	if _, err := tx.Exec("UPDATE oauth_codes SET used=1 WHERE token_hash=?", HashToken(code)); err != nil {
		return OAuthTokens{}, err
	}
	grantID, err := RandomSecret("grant_")
	if err != nil {
		return OAuthTokens{}, err
	}
	var name string
	if err := tx.QueryRow("SELECT name FROM oauth_clients WHERE id=?", clientID).Scan(&name); err != nil {
		return OAuthTokens{}, err
	}
	if _, err := tx.Exec("INSERT INTO oauth_grants(id,user_id,client_id,name,scopes,created_at) VALUES(?,?,?,?,?,?)", grantID, userID, clientID, name, scopes, time.Now().Unix()); err != nil {
		return OAuthTokens{}, err
	}
	tokens, err := issueOAuthTokens(tx, userID, clientID, grantID, scopes)
	if err != nil {
		return OAuthTokens{}, err
	}
	if err := tx.Commit(); err != nil {
		return OAuthTokens{}, err
	}
	return tokens, nil
}

func (s *Store) RefreshOAuth(refresh, clientID string) (OAuthTokens, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return OAuthTokens{}, err
	}
	defer tx.Rollback()
	var userID, registeredClient, grantID, scopes string
	var expiry, revoked, grantRevoked int64
	err = tx.QueryRow(`SELECT c.user_id,c.client_id,c.grant_id,c.scopes,c.expires_at,c.revoked_at,g.revoked_at FROM credentials c JOIN oauth_grants g ON g.id=c.grant_id WHERE c.token_hash=? AND c.kind='refresh'`, HashToken(refresh)).Scan(&userID, &registeredClient, &grantID, &scopes, &expiry, &revoked, &grantRevoked)
	if errors.Is(err, sql.ErrNoRows) {
		return OAuthTokens{}, ErrUnauthorized
	}
	if err != nil {
		return OAuthTokens{}, err
	}
	if registeredClient != clientID || expiry <= time.Now().Unix() || grantRevoked != 0 {
		return OAuthTokens{}, ErrUnauthorized
	}
	if revoked != 0 {
		if _, err := tx.Exec("UPDATE oauth_grants SET revoked_at=? WHERE id=?", time.Now().Unix(), grantID); err != nil {
			return OAuthTokens{}, err
		}
		if err := tx.Commit(); err != nil {
			return OAuthTokens{}, err
		}
		return OAuthTokens{}, ErrUnauthorized
	}
	if _, err := tx.Exec("UPDATE credentials SET revoked_at=? WHERE token_hash=?", time.Now().Unix(), HashToken(refresh)); err != nil {
		return OAuthTokens{}, err
	}
	tokens, err := issueOAuthTokens(tx, userID, clientID, grantID, scopes)
	if err != nil {
		return OAuthTokens{}, err
	}
	if err := tx.Commit(); err != nil {
		return OAuthTokens{}, err
	}
	return tokens, nil
}

func (s *Store) OAuthGrants(userID string) ([]OAuthGrant, error) {
	rows, err := s.db.Query("SELECT id,name,scopes,created_at,revoked_at FROM oauth_grants WHERE user_id=? ORDER BY created_at DESC", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []OAuthGrant{}
	for rows.Next() {
		var g OAuthGrant
		if err := rows.Scan(&g.ID, &g.Name, &g.Scopes, &g.CreatedAt, &g.RevokedAt); err != nil {
			return nil, err
		}
		items = append(items, g)
	}
	return items, rows.Err()
}

func (s *Store) RevokeGrant(userID, id string) (bool, error) {
	result, err := s.db.Exec("UPDATE oauth_grants SET revoked_at=? WHERE user_id=? AND id=? AND revoked_at=0", time.Now().Unix(), userID, id)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n > 0, err
}
