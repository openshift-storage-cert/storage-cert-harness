package safefs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadRelativeRejectsPathTraversal(t *testing.T) {
	if _, err := ReadRelative(".", "../go.mod"); err == nil {
		t.Fatal("expected path traversal to be rejected")
	}
}

func TestReadUnderRejectsEscape(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadUnder(root, outside); err == nil {
		t.Fatal("expected path outside root to be rejected")
	}
}

func TestWriteUnderRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	if err := WriteUnder(root, "../escape.txt", []byte("x"), 0o600); err == nil {
		t.Fatal("expected traversal write to be rejected")
	}
}

func TestMkdirAllAndReadWrite(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "a", "b")
	if err := MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "file.txt")
	if err := WriteFile(path, []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	data, err := ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "ok" {
		t.Fatalf("got %q", data)
	}
}

func TestReadUnderSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	secret := filepath.Join(root, "outside.txt")
	if err := os.WriteFile(secret, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link.txt")
	if err := os.Symlink(secret, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := ReadUnder(root, link); err == nil {
		t.Fatal("expected symlink escape to be rejected")
	}
}
