package store

import (
	"testing"
	"time"
)

func TestUpdateOwnershipAndMetadata(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.db.Close() })
	owner := HashToken("owner")
	id, err := s.Create("original", owner, "ip")
	if err != nil {
		t.Fatal(err)
	}
	before, _ := s.GetByID(id)
	for _, tc := range []struct {
		name, id, token string
	}{
		{"wrong owner", id, HashToken("other")},
		{"missing document", "doesNotExist", owner},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ok, err := s.Update(tc.id, "unauthorized", tc.token)
			if err != nil || ok {
				t.Fatalf("update = %v, %v", ok, err)
			}
		})
	}
	unchanged, _ := s.GetByID(id)
	if unchanged.Content != "original" {
		t.Fatal("rejected update changed content")
	}
	for _, content := range []string{"# Updated\n\n中文 \\ path\n", "# Updated\n\n中文 \\ path\n", "last save"} {
		ok, err := s.Update(id, content, owner)
		if err != nil || !ok {
			t.Fatalf("update = %v, %v", ok, err)
		}
		after, _ := s.GetByID(id)
		if after.Content != content || after.ID != before.ID || after.CreatedAt != before.CreatedAt || after.TokenHash != before.TokenHash {
			t.Fatalf("content or metadata mismatch: %+v", after)
		}
	}
	docs, err := s.ListByTokenHash(owner)
	if err != nil || len(docs) != 1 {
		t.Fatalf("update created an extra document: %v, %v", docs, err)
	}
	if _, err := s.Delete(id, owner); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.Update(id, "resurrected", owner); err != nil || ok {
		t.Fatalf("deleted document update = %v, %v", ok, err)
	}
}

func TestUpdateDoesNotReviveExpiredDocument(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.db.Close() })
	owner := HashToken("owner")
	id, err := s.Create("expired content", owner, "ip")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec("UPDATE documents SET expires_at = ? WHERE id = ?", time.Now().Unix()-1, id); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.Update(id, "new content", owner); err != nil || ok {
		t.Fatalf("expired update = %v, %v", ok, err)
	}
	var content string
	if err := s.db.QueryRow("SELECT content FROM documents WHERE id = ?", id).Scan(&content); err != nil || content != "expired content" {
		t.Fatalf("expired content changed: %q, %v", content, err)
	}
	s.db.Close()
	if _, err := s.Update(id, "failure", owner); err == nil {
		t.Fatal("database failure should be returned")
	}
}
