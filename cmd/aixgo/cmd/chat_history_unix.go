//go:build !windows

package cmd

import (
	"os"
	"syscall"
)

// openChatHistoryForRead opens the chat history file read-only with
// O_NOFOLLOW so that a symlink replacement (e.g. an attacker pointing
// ~/.aixgo/chat_history at another file on the system) is rejected with
// ELOOP rather than silently followed. This is defense-in-depth against
// local write access to the user's dotfiles directory; it does not
// protect against an attacker who can already overwrite files as the
// user, but it prevents the readline layer from being tricked into
// slurping arbitrary file contents into its in-memory history buffer
// (which would then be rewritten back to chat_history on shutdown).
//
// #nosec G304 -- path is constructed from os.UserHomeDir() and a fixed
// relative path (.aixgo/chat_history); not influenced by untrusted input.
func openChatHistoryForRead(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
}
