package file

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestValidatePath covers the #131 hardening of pkg/tools/file/read.go. The
// contract is:
//  1. Empty paths and null bytes are rejected.
//  2. The absolute, cleaned path must fall inside a non-empty allowlist
//     (cwd, $HOME, /usr/local, /etc, /tmp, /var/folders, $TMPDIR).
//  3. If the path already exists, symlinks are resolved and the real target
//     must also live inside the allowlist (symlink-escape defence).
func TestValidatePath(t *testing.T) {
	// Anchor to an allowlisted tempdir so ValidatePath() accepts us.
	tmpDir := t.TempDir()
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(prevWD) })

	// Materialise a real file inside the allowlist so relative paths resolve.
	inside := filepath.Join(tmpDir, "inside.txt")
	if err := os.WriteFile(inside, []byte("ok"), 0o600); err != nil {
		t.Fatalf("write inside: %v", err)
	}

	// Symlink that points outside every allowlist entry. We need a target
	// that (a) exists on both linux and darwin runners, (b) is NOT inside
	// cwd, $HOME, /usr/local, /etc, /tmp, /var/folders, or $TMPDIR.
	//
	// /etc/hosts won't work because /etc is explicitly on the allowlist
	// (even though it looks "system-y"). /bin/sh is a reliable choice:
	// it exists on every Unix and /bin is not an allowlist root.
	// On linux with usrmerge /bin is a symlink to /usr/bin, which still
	// resolves outside the allowlist (only /usr/local is allowed).
	evilTarget := "/bin/sh"
	if _, statErr := os.Lstat(evilTarget); statErr != nil {
		evilTarget = ""
	}
	evil := filepath.Join(tmpDir, "evil.link")
	if evilTarget != "" {
		if err := os.Symlink(evilTarget, evil); err != nil {
			t.Logf("symlink skipped: %v", err)
			evil = ""
		}
	} else {
		evil = ""
	}

	tests := []struct {
		name    string
		path    string
		skip    bool
		wantErr bool
	}{
		{name: "empty rejected", path: "", wantErr: true},
		{name: "null byte rejected", path: "a\x00b", wantErr: true},
		{name: "relative inside cwd accepted", path: "inside.txt", wantErr: false},
		{name: "absolute inside tmp accepted", path: inside, wantErr: false},
		{
			name: "absolute outside allowlist rejected",
			// /root is outside every allowlist entry on macOS and most linux.
			path:    "/root/.ssh/id_rsa",
			wantErr: true,
			skip:    runtime.GOOS == "windows",
		},
		{
			name:    "symlink escape rejected",
			path:    evil,
			wantErr: true,
			skip:    evil == "",
		},
		{
			name:    "parent-traversal-then-inside still resolves to inside",
			path:    filepath.Join(tmpDir, "sub", "..", "inside.txt"),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip {
				t.Skip("not applicable on this platform")
			}
			err := ValidatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath(%q) err = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

// TestWriteFileHandler_Permissions guards the B fix (gosec G306 #120 and
// G301 #119). writeFileHandler must create parent directories with 0o750
// and write files with 0o600, regardless of umask.
func TestWriteFileHandler_Permissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX mode bits not meaningful on windows")
	}

	tmpDir := t.TempDir()

	// Target lives inside a nested subdirectory that writeFileHandler will
	// create via MkdirAll — that's what we want to assert perms on.
	nested := filepath.Join(tmpDir, "nested", "deeper")
	target := filepath.Join(nested, "agent-output.txt")

	// The handler calls ValidatePath, so we must chdir into an allowlist
	// root. tmpDir (inside os.TempDir()) is already covered.
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(prevWD) })

	_, err = writeFileHandler(context.Background(), map[string]any{
		"path":    target,
		"content": "super secret tool output",
	})
	if err != nil {
		t.Fatalf("writeFileHandler: %v", err)
	}

	// Verify the directory was created with mode <=0o750.
	dirInfo, err := os.Stat(nested)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	dirMode := dirInfo.Mode().Perm()
	if dirMode&^0o750 != 0 {
		t.Errorf("dir mode = %04o, want <=0o750 (G301)", dirMode)
	}

	// Verify the file was written with mode <=0o600.
	fileInfo, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	fileMode := fileInfo.Mode().Perm()
	if fileMode&^0o600 != 0 {
		t.Errorf("file mode = %04o, want <=0o600 (G306)", fileMode)
	}
}

// TestWriteFileHandler_RejectsTraversal ensures the write path inherits the
// ValidatePath guard so an agent cannot coerce writes outside the allowlist.
func TestWriteFileHandler_RejectsTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(prevWD) })

	_, err = writeFileHandler(context.Background(), map[string]any{
		"path":    "/root/.ssh/authorized_keys",
		"content": "attacker-key",
	})
	if err == nil {
		t.Fatal("writeFileHandler accepted /root path, want rejection")
	}
	if !strings.Contains(err.Error(), "outside allowed") && !strings.Contains(err.Error(), "path") {
		t.Errorf("err = %q, want an allowlist-rejection message", err.Error())
	}
}
