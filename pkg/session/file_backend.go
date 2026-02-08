package session

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// ErrInvalidPathComponent is returned when a path component contains unsafe characters.
var ErrInvalidPathComponent = errors.New("invalid path component: contains path separator or traversal sequence")

// validatePathComponent checks that a string is safe to use as a path component.
// It rejects empty strings, path separators, and traversal sequences.
func validatePathComponent(s string) error {
	if s == "" {
		return errors.New("path component cannot be empty")
	}
	if strings.ContainsAny(s, `/\`) || strings.Contains(s, "..") {
		return ErrInvalidPathComponent
	}
	return nil
}

// FileBackend implements StorageBackend using JSONL files.
// Storage layout:
//
//	~/.aixgo/sessions/
//	  └── <agent-name>/
//	      ├── sessions.json          # Session index
//	      ├── <session-id>.jsonl     # Session entries
//	      └── checkpoints/
//	          └── <checkpoint-id>.json
type FileBackend struct {
	baseDir string
	mu      sync.RWMutex
	closed  bool
}

// NewFileBackend creates a new file-based storage backend.
// If baseDir is empty, uses ~/.aixgo/sessions.
func NewFileBackend(baseDir string) (*FileBackend, error) {
	if baseDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home directory: %w", err)
		}
		baseDir = filepath.Join(home, ".aixgo", "sessions")
	}

	// Ensure base directory exists
	if err := os.MkdirAll(baseDir, 0700); err != nil {
		return nil, fmt.Errorf("create base directory: %w", err)
	}

	return &FileBackend{
		baseDir: baseDir,
	}, nil
}

// SaveSession creates or updates session metadata.
func (f *FileBackend) SaveSession(ctx context.Context, meta *SessionMetadata) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return ErrStorageClosed
	}

	// Validate path components to prevent path traversal
	if err := validatePathComponent(meta.AgentName); err != nil {
		return fmt.Errorf("invalid agent name: %w", err)
	}
	if err := validatePathComponent(meta.ID); err != nil {
		return fmt.Errorf("invalid session ID: %w", err)
	}

	// Ensure agent directory exists
	agentDir := filepath.Join(f.baseDir, meta.AgentName)
	if err := os.MkdirAll(agentDir, 0700); err != nil {
		return fmt.Errorf("create agent directory: %w", err)
	}

	// Load existing index
	indexPath := filepath.Join(agentDir, "sessions.json")
	index := make(map[string]*SessionMetadata)

	data, err := os.ReadFile(indexPath) // #nosec G304 - path components validated to prevent traversal
	if err == nil {
		if err := json.Unmarshal(data, &index); err != nil {
			return fmt.Errorf("parse sessions index: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read sessions index: %w", err)
	}

	// Update index
	index[meta.ID] = meta

	// Write index
	data, err = json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal sessions index: %w", err)
	}

	if err := os.WriteFile(indexPath, data, 0600); err != nil {
		return fmt.Errorf("write sessions index: %w", err)
	}

	return nil
}

// LoadSession retrieves session metadata by ID.
func (f *FileBackend) LoadSession(ctx context.Context, sessionID string) (*SessionMetadata, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.closed {
		return nil, ErrStorageClosed
	}

	// Validate session ID to prevent path traversal
	if err := validatePathComponent(sessionID); err != nil {
		return nil, fmt.Errorf("invalid session ID: %w", err)
	}

	// Search all agent directories for the session
	entries, err := os.ReadDir(f.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("read base directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		indexPath := filepath.Join(f.baseDir, entry.Name(), "sessions.json")
		data, err := os.ReadFile(indexPath) // #nosec G304 - path components validated to prevent traversal
		if err != nil {
			continue
		}

		index := make(map[string]*SessionMetadata)
		if err := json.Unmarshal(data, &index); err != nil {
			continue
		}

		if meta, ok := index[sessionID]; ok {
			return meta, nil
		}
	}

	return nil, ErrSessionNotFound
}

// DeleteSession removes a session and all its entries.
func (f *FileBackend) DeleteSession(ctx context.Context, sessionID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return ErrStorageClosed
	}

	// Find the session first
	meta, err := f.loadSessionUnlocked(sessionID)
	if err != nil {
		return err
	}

	agentDir := filepath.Join(f.baseDir, meta.AgentName)

	// Remove entries file
	entriesPath := filepath.Join(agentDir, sessionID+".jsonl")
	_ = os.Remove(entriesPath) // Ignore if doesn't exist

	// Remove from index
	indexPath := filepath.Join(agentDir, "sessions.json")
	data, err := os.ReadFile(indexPath) // #nosec G304 - path components validated to prevent traversal
	if err != nil {
		return fmt.Errorf("read sessions index: %w", err)
	}

	index := make(map[string]*SessionMetadata)
	if err := json.Unmarshal(data, &index); err != nil {
		return fmt.Errorf("parse sessions index: %w", err)
	}

	delete(index, sessionID)

	data, err = json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal sessions index: %w", err)
	}

	if err := os.WriteFile(indexPath, data, 0600); err != nil {
		return fmt.Errorf("write sessions index: %w", err)
	}

	return nil
}

// ListSessions returns sessions for an agent matching the filter options.
func (f *FileBackend) ListSessions(ctx context.Context, agentName string, opts ListOptions) ([]*SessionMetadata, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.closed {
		return nil, ErrStorageClosed
	}

	// Validate agent name to prevent path traversal
	if err := validatePathComponent(agentName); err != nil {
		return nil, fmt.Errorf("invalid agent name: %w", err)
	}

	agentDir := filepath.Join(f.baseDir, agentName)
	indexPath := filepath.Join(agentDir, "sessions.json")

	data, err := os.ReadFile(indexPath) // #nosec G304 - path validated above
	if err != nil {
		if os.IsNotExist(err) {
			return []*SessionMetadata{}, nil
		}
		return nil, fmt.Errorf("read sessions index: %w", err)
	}

	index := make(map[string]*SessionMetadata)
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("parse sessions index: %w", err)
	}

	// Filter and collect sessions
	var sessions []*SessionMetadata
	for _, meta := range index {
		// Filter by user ID if specified
		if opts.UserID != "" && meta.UserID != opts.UserID {
			continue
		}
		sessions = append(sessions, meta)
	}

	// Sort by updated time (most recent first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	// Apply offset and limit
	if opts.Offset > 0 {
		if opts.Offset >= len(sessions) {
			return []*SessionMetadata{}, nil
		}
		sessions = sessions[opts.Offset:]
	}

	if opts.Limit > 0 && opts.Limit < len(sessions) {
		sessions = sessions[:opts.Limit]
	}

	return sessions, nil
}

// AppendEntry adds an entry to a session (append-only).
func (f *FileBackend) AppendEntry(ctx context.Context, sessionID string, entry *SessionEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return ErrStorageClosed
	}

	// Find the session to get the agent name
	meta, err := f.loadSessionUnlocked(sessionID)
	if err != nil {
		return err
	}

	agentDir := filepath.Join(f.baseDir, meta.AgentName)
	entriesPath := filepath.Join(agentDir, sessionID+".jsonl")

	// Open file for append
	file, err := os.OpenFile(entriesPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600) // #nosec G304 - path components validated to prevent traversal
	if err != nil {
		return fmt.Errorf("open entries file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Write entry as JSON line
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal entry: %w", err)
	}

	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write entry: %w", err)
	}

	return nil
}

// LoadEntries retrieves all entries for a session in order.
func (f *FileBackend) LoadEntries(ctx context.Context, sessionID string) ([]*SessionEntry, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.closed {
		return nil, ErrStorageClosed
	}

	// Find the session to get the agent name
	meta, err := f.loadSessionUnlocked(sessionID)
	if err != nil {
		return nil, err
	}

	agentDir := filepath.Join(f.baseDir, meta.AgentName)
	entriesPath := filepath.Join(agentDir, sessionID+".jsonl")

	file, err := os.Open(entriesPath) // #nosec G304 - path components validated to prevent traversal
	if err != nil {
		if os.IsNotExist(err) {
			return []*SessionEntry{}, nil
		}
		return nil, fmt.Errorf("open entries file: %w", err)
	}
	defer func() { _ = file.Close() }()

	var entries []*SessionEntry
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry SessionEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return nil, fmt.Errorf("parse entry: %w", err)
		}
		entries = append(entries, &entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan entries: %w", err)
	}

	return entries, nil
}

// SaveCheckpoint stores a checkpoint.
func (f *FileBackend) SaveCheckpoint(ctx context.Context, checkpoint *Checkpoint) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return ErrStorageClosed
	}

	// Validate checkpoint ID to prevent path traversal
	if err := validatePathComponent(checkpoint.ID); err != nil {
		return fmt.Errorf("invalid checkpoint ID: %w", err)
	}

	// Find the session to get the agent name (validates sessionID)
	meta, err := f.loadSessionUnlocked(checkpoint.SessionID)
	if err != nil {
		return err
	}

	checkpointsDir := filepath.Join(f.baseDir, meta.AgentName, "checkpoints")
	if err := os.MkdirAll(checkpointsDir, 0700); err != nil {
		return fmt.Errorf("create checkpoints directory: %w", err)
	}

	checkpointPath := filepath.Join(checkpointsDir, checkpoint.ID+".json")

	data, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal checkpoint: %w", err)
	}

	if err := os.WriteFile(checkpointPath, data, 0600); err != nil {
		return fmt.Errorf("write checkpoint: %w", err)
	}

	return nil
}

// LoadCheckpoint retrieves a checkpoint by ID.
func (f *FileBackend) LoadCheckpoint(ctx context.Context, checkpointID string) (*Checkpoint, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.closed {
		return nil, ErrStorageClosed
	}

	// Validate checkpoint ID to prevent path traversal
	if err := validatePathComponent(checkpointID); err != nil {
		return nil, fmt.Errorf("invalid checkpoint ID: %w", err)
	}

	// Search all agent directories for the checkpoint
	entries, err := os.ReadDir(f.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrCheckpointNotFound
		}
		return nil, fmt.Errorf("read base directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		checkpointPath := filepath.Join(f.baseDir, entry.Name(), "checkpoints", checkpointID+".json")
		data, err := os.ReadFile(checkpointPath) // #nosec G304 - path components validated to prevent traversal
		if err != nil {
			continue
		}

		var checkpoint Checkpoint
		if err := json.Unmarshal(data, &checkpoint); err != nil {
			continue
		}

		return &checkpoint, nil
	}

	return nil, ErrCheckpointNotFound
}

// Close releases any resources held by the backend.
func (f *FileBackend) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.closed = true
	return nil
}

// loadSessionUnlocked is an internal helper that loads session without acquiring locks.
// Caller must hold appropriate lock.
func (f *FileBackend) loadSessionUnlocked(sessionID string) (*SessionMetadata, error) {
	// Validate session ID to prevent path traversal
	if err := validatePathComponent(sessionID); err != nil {
		return nil, fmt.Errorf("invalid session ID: %w", err)
	}

	// Search all agent directories for the session
	entries, err := os.ReadDir(f.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("read base directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		indexPath := filepath.Join(f.baseDir, entry.Name(), "sessions.json")
		data, err := os.ReadFile(indexPath) // #nosec G304 - path components validated to prevent traversal
		if err != nil {
			continue
		}

		index := make(map[string]*SessionMetadata)
		if err := json.Unmarshal(data, &index); err != nil {
			continue
		}

		if meta, ok := index[sessionID]; ok {
			return meta, nil
		}
	}

	return nil, ErrSessionNotFound
}
