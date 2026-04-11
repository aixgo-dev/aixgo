package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrInvalidSessionID is returned when a caller supplies a session identifier
// that is empty, too short, or does not match the canonical 12-char hex format
// produced by generateID.
var ErrInvalidSessionID = errors.New("invalid session id")

// minSessionIDLen is the minimum length accepted by lookup operations that
// support prefix matching (Get, Delete). Shorter prefixes are rejected to
// avoid accidental broad matches.
const minSessionIDLen = 6

// sessionIDPattern is the accepted format for session IDs persisted to disk:
// 11 or 12 lowercase hex characters. New IDs produced by generateID are always
// 12 characters; the 11-character form is accepted for backward compatibility
// with sessions created by earlier releases (which stripped a UUID dash from a
// 12-char slice and yielded an 11-char ID). Either width still rejects path
// separators, dots, and any non-hex byte, which is what the validation exists
// to prevent.
var sessionIDPattern = regexp.MustCompile(`^[a-f0-9]{11,12}$`)

// Manager handles session CRUD operations.
type Manager struct {
	sessionsDir string
}

// NewManager creates a new session manager.
func NewManager() (*Manager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	sessionsDir := filepath.Join(homeDir, ".aixgo", "sessions")
	if err := os.MkdirAll(sessionsDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create sessions directory: %w", err)
	}

	return &Manager{
		sessionsDir: sessionsDir,
	}, nil
}

// Create creates a new session with the given model.
func (m *Manager) Create(model string) (*Session, error) {
	workingDir, _ := os.Getwd()

	sess := &Session{
		ID:         generateID(),
		Model:      model,
		Messages:   []Message{},
		TotalCost:  0,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		WorkingDir: workingDir,
		Metadata:   make(map[string]string),
	}

	if err := m.Save(sess); err != nil {
		return nil, err
	}

	return sess, nil
}

// Get retrieves a session by ID.
func (m *Manager) Get(id string) (*Session, error) {
	if id == "" || len(id) < minSessionIDLen {
		return nil, ErrInvalidSessionID
	}
	// Support partial ID matching
	matches, err := m.findByPrefix(id)
	if err != nil {
		return nil, err
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("session not found: %s", id)
	}

	if len(matches) > 1 {
		return nil, fmt.Errorf("ambiguous session ID: %s (matches %d sessions)", id, len(matches))
	}

	return m.loadSession(matches[0])
}

// Save persists a session to disk.
func (m *Manager) Save(sess *Session) error {
	if sess == nil || !sessionIDPattern.MatchString(sess.ID) {
		return ErrInvalidSessionID
	}
	sess.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	path := m.sessionPath(sess.ID)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	return nil
}

// Delete removes a session by ID.
func (m *Manager) Delete(id string) error {
	if id == "" || len(id) < minSessionIDLen {
		return ErrInvalidSessionID
	}
	// Support partial ID matching
	matches, err := m.findByPrefix(id)
	if err != nil {
		return err
	}

	if len(matches) == 0 {
		return fmt.Errorf("session not found: %s", id)
	}

	if len(matches) > 1 {
		return fmt.Errorf("ambiguous session ID: %s (matches %d sessions)", id, len(matches))
	}

	path := m.sessionPath(matches[0])
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	return nil
}

// List returns all sessions sorted by updated time (newest first).
func (m *Manager) List() ([]*Session, error) {
	entries, err := os.ReadDir(m.sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Session{}, nil
		}
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	var sessions []*Session
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		id := strings.TrimSuffix(entry.Name(), ".json")
		sess, err := m.loadSession(id)
		if err != nil {
			// Skip corrupt sessions
			continue
		}
		sessions = append(sessions, sess)
	}

	// Sort by updated time (newest first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	return sessions, nil
}

// loadSession loads a session from disk.
func (m *Manager) loadSession(id string) (*Session, error) {
	path := m.sessionPath(id)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &sess, nil
}

// findByPrefix finds session IDs matching a prefix.
func (m *Manager) findByPrefix(prefix string) ([]string, error) {
	entries, err := os.ReadDir(m.sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	var matches []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		id := strings.TrimSuffix(entry.Name(), ".json")
		if strings.HasPrefix(id, prefix) {
			matches = append(matches, id)
		}
	}

	return matches, nil
}

// sessionPath returns the file path for a session ID.
func (m *Manager) sessionPath(id string) string {
	return filepath.Join(m.sessionsDir, id+".json")
}

// generateID generates a short unique session ID consisting of exactly 12
// lowercase hex characters. The format is enforced by sessionIDPattern and
// validated on Save.
func generateID() string {
	// A uuid v4 string contains dashes at fixed positions; strip them first so
	// the leading 12 characters are always pure hex.
	hex := strings.ReplaceAll(uuid.New().String(), "-", "")
	return hex[:12]
}
