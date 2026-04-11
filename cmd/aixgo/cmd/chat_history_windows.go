//go:build windows

package cmd

import "os"

// openChatHistoryForRead opens the chat history file read-only. Windows
// does not expose an O_NOFOLLOW equivalent via the os package, so this
// falls back to a plain open. The file is still scoped to %USERPROFILE%
// and written with 0o600 on Unix; Windows users do not benefit from the
// symlink-rejection mitigation but are no worse off than before.
//
// #nosec G304 -- path is constructed from os.UserHomeDir() and a fixed
// relative path (.aixgo/chat_history); not influenced by untrusted input.
func openChatHistoryForRead(path string) (*os.File, error) {
	return os.Open(path)
}
