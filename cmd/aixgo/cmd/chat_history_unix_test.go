//go:build !windows

package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// TestOpenChatHistoryForRead_RejectsSymlink verifies the O_NOFOLLOW
// mitigation in openChatHistoryForRead: a history path that has been
// replaced with a symlink to another file must be rejected with ELOOP
// rather than silently followed (which would let the readline layer
// slurp arbitrary file contents into its history buffer).
func TestOpenChatHistoryForRead_RejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("sensitive\n"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}

	link := filepath.Join(dir, "chat_history")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	f, err := openChatHistoryForRead(link)
	if err == nil {
		_ = f.Close()
		t.Fatal("openChatHistoryForRead followed a symlink; expected ELOOP rejection")
	}
	if !errors.Is(err, syscall.ELOOP) {
		t.Errorf("expected ELOOP, got %v", err)
	}
}

// TestOpenChatHistoryForRead_RegularFile verifies the happy path still
// works for a non-symlink history file.
func TestOpenChatHistoryForRead_RegularFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chat_history")
	if err := os.WriteFile(path, []byte("hello\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	f, err := openChatHistoryForRead(path)
	if err != nil {
		t.Fatalf("openChatHistoryForRead: %v", err)
	}
	_ = f.Close()
}

// TestOpenChatHistoryForRead_Missing verifies ENOENT is returned (and
// therefore classified by the caller as "normal first run") for a
// nonexistent history path.
func TestOpenChatHistoryForRead_Missing(t *testing.T) {
	dir := t.TempDir()
	f, err := openChatHistoryForRead(filepath.Join(dir, "does_not_exist"))
	if err == nil {
		_ = f.Close()
		t.Fatal("expected error for missing file")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected os.ErrNotExist, got %v", err)
	}
}
