package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnv(t *testing.T) {
	t.Setenv("FASTMD_ENV_KEEP", "existing")
	path := filepath.Join(t.TempDir(), ".env")
	content := "# private config\nFASTMD_ENV_KEEP=file\nFASTMD_ENV_LITERAL='$(no-command) `literal`'\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Unsetenv("FASTMD_ENV_LITERAL") })
	if err := LoadEnv(path); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("FASTMD_ENV_KEEP") != "existing" || os.Getenv("FASTMD_ENV_LITERAL") != "$(no-command) `literal`" {
		t.Fatal("configuration modified unexpectedly")
	}
	if err := LoadEnv(path + ".missing"); err != nil {
		t.Fatal(err)
	}
}
