package service

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/wonfull888/fastmd/internal/store"
)

type Error struct {
	Code    string `json:"code"`
	Message string `json:"error"`
	Status  int    `json:"-"`
}

func (e *Error) Error() string                       { return e.Code + ": " + e.Message }
func Failure(code, message string, status int) error { return &Error{code, message, status} }
func Describe(err error) *Error {
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	if errors.Is(err, store.ErrConflict) {
		return &Error{"IDEMPOTENCY_CONFLICT", err.Error(), 409}
	}
	if errors.Is(err, store.ErrGone) {
		return &Error{"DOCUMENT_GONE", err.Error(), 410}
	}
	return &Error{"INTERNAL_ERROR", "internal server error", 500}
}

type Documents struct {
	Store   *store.Store
	BaseURL string
}
type Document struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	Title     string `json:"title,omitempty"`
	Content   string `json:"content,omitempty"`
	CreatedAt int64  `json:"created_at,omitempty"`
}
type Listing struct {
	Documents  []Document `json:"documents"`
	NextCursor string     `json:"next_cursor,omitempty"`
}
type Usage struct {
	Account       store.User `json:"account"`
	DocumentCount int        `json:"document_count"`
}

var validID = regexp.MustCompile(`^[a-zA-Z0-9]{1,64}$`)
var validRequest = regexp.MustCompile(`^[a-zA-Z0-9._:-]{1,128}$`)

func Allowed(p store.Credential, scope string) error {
	if p.User.ID == "" {
		return Failure("AUTH_REQUIRED", "sign in or configure an issued API token", 401)
	}
	if !p.Allows(scope) {
		return Failure("FORBIDDEN", "credential does not allow this operation", 403)
	}
	return nil
}
func Content(content string) error {
	if strings.TrimSpace(content) == "" {
		return Failure("INVALID_CONTENT", "content is empty", 400)
	}
	if len(content) > 1024*1024 {
		return Failure("CONTENT_TOO_LARGE", "content exceeds 1 MiB", 413)
	}
	return nil
}
func Title(content, fallback string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "# ") {
			title := strings.TrimSpace(strings.TrimPrefix(line, "# "))
			if title != "" {
				return title
			}
		}
	}
	return fallback
}
func (d *Documents) info(doc *store.Document, content bool) Document {
	result := Document{ID: doc.ID, URL: strings.TrimRight(d.BaseURL, "/") + "/" + doc.ID + ".md", Title: Title(doc.Content, doc.ID), CreatedAt: doc.CreatedAt}
	if content {
		result.Content = doc.Content
	}
	return result
}
func (d *Documents) Create(p store.Credential, content, requestID string) (Document, error) {
	if err := Allowed(p, "write"); err != nil {
		return Document{}, err
	}
	if err := Content(content); err != nil {
		return Document{}, err
	}
	if requestID != "" && !validRequest.MatchString(requestID) {
		return Document{}, Failure("INVALID_REQUEST_ID", "request_id must be 1-128 letters, numbers, dots, underscores, colons or hyphens", 400)
	}
	id, err := d.Store.CreateOwned(p.User.ID, content, requestID)
	if err != nil {
		return Document{}, err
	}
	doc, err := d.Store.GetByID(id)
	if err != nil {
		return Document{}, err
	}
	if doc == nil {
		return Document{}, store.ErrGone
	}
	return d.info(doc, false), nil
}
func (d *Documents) Update(p store.Credential, id, content string) (Document, error) {
	if err := Allowed(p, "write"); err != nil {
		return Document{}, err
	}
	if err := Content(content); err != nil {
		return Document{}, err
	}
	if !validID.MatchString(id) {
		return Document{}, Failure("INVALID_ID", "invalid document ID", 400)
	}
	ok, err := d.Store.UpdateOwned(p.User.ID, id, content)
	if err != nil {
		return Document{}, err
	}
	if !ok {
		return Document{}, Failure("NOT_FOUND", "document not found or not owned by this account", 404)
	}
	doc, err := d.Store.GetByID(id)
	if err != nil {
		return Document{}, err
	}
	if doc == nil {
		return Document{}, Failure("NOT_FOUND", "document no longer exists", 404)
	}
	return d.info(doc, false), nil
}
func (d *Documents) Get(p store.Credential, id string) (Document, error) {
	if err := Allowed(p, "read"); err != nil {
		return Document{}, err
	}
	if !validID.MatchString(id) {
		return Document{}, Failure("INVALID_ID", "invalid document ID", 400)
	}
	doc, err := d.Store.GetByID(id)
	if err != nil {
		return Document{}, err
	}
	if doc == nil {
		return Document{}, Failure("NOT_FOUND", "document not found", 404)
	}
	return d.info(doc, true), nil
}
func (d *Documents) Delete(p store.Credential, id string) error {
	if err := Allowed(p, "delete"); err != nil {
		return err
	}
	if !validID.MatchString(id) {
		return Failure("INVALID_ID", "invalid document ID", 400)
	}
	ok, err := d.Store.DeleteOwned(p.User.ID, id)
	if err != nil {
		return err
	}
	if !ok {
		return Failure("NOT_FOUND", "document not found or not owned by this account", 404)
	}
	return nil
}
func (d *Documents) List(p store.Credential, offset, limit int) (Listing, error) {
	if err := Allowed(p, "read"); err != nil {
		return Listing{}, err
	}
	if offset < 0 || limit < 1 || limit > 100 {
		return Listing{}, Failure("INVALID_PAGE", "limit must be 1-100 and cursor non-negative", 400)
	}
	docs, err := d.Store.ListOwned(p.User.ID, offset, limit+1)
	if err != nil {
		return Listing{}, err
	}
	result := Listing{Documents: []Document{}}
	if len(docs) > limit {
		result.NextCursor = fmt.Sprint(offset + limit)
		docs = docs[:limit]
	}
	for i := range docs {
		result.Documents = append(result.Documents, d.info(&docs[i], false))
	}
	return result, nil
}
func (d *Documents) Usage(p store.Credential) (Usage, error) {
	if err := Allowed(p, "read"); err != nil {
		return Usage{}, err
	}
	count, err := d.Store.DocumentCount(p.User.ID)
	return Usage{Account: p.User, DocumentCount: count}, err
}
