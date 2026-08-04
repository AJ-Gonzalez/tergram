package tgc

import (
	"os"
	"path/filepath"
	"testing"
)

// withStdin points os.Stdin at a pipe feeding `input` and restores it after f.
func withStdin(t *testing.T, input string, f func()) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString(input); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old }()
	f()
}

func writeSession(t *testing.T, dir string) string {
	t.Helper()
	p := filepath.Join(dir, "session.json")
	if err := os.WriteFile(p, []byte("not-a-real-session"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPromptStaleSessionClearsOnYes(t *testing.T) {
	p := writeSession(t, t.TempDir())
	var cleaned bool
	var err error
	withStdin(t, "y\n", func() {
		cleaned, err = promptStaleSession(p)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cleaned {
		t.Fatal("expected prompt to clear the session on 'y'")
	}
	if fileExists(p) {
		t.Fatal("session file should have been removed")
	}
}

func TestPromptStaleSessionKeepsOnNo(t *testing.T) {
	p := writeSession(t, t.TempDir())
	var cleaned bool
	var err error
	withStdin(t, "N\n", func() {
		cleaned, err = promptStaleSession(p)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cleaned {
		t.Fatal("expected prompt NOT to clear the session on 'n'")
	}
	if !fileExists(p) {
		t.Fatal("session file should have been kept")
	}
}

func TestFileExists(t *testing.T) {
	dir := t.TempDir()
	if !fileExists(dir) {
		t.Fatal("directory should exist")
	}
	if fileExists(filepath.Join(dir, "nope")) {
		t.Fatal("missing path should not exist")
	}
}
