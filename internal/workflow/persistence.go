package workflow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

// safeIDPattern defines the allowed characters for execution and checkpoint IDs
// Only alphanumeric characters, hyphens, and underscores are allowed
var safeIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// validateID validates that an ID is safe and does not contain path traversal characters
func validateID(id string) error {
	if id == "" {
		return fmt.Errorf("ID cannot be empty")
	}
	if len(id) > 256 {
		return fmt.Errorf("ID too long (max 256 characters)")
	}
	if !safeIDPattern.MatchString(id) {
		return fmt.Errorf("ID contains invalid characters: only alphanumeric, hyphens, and underscores allowed")
	}
	return nil
}

// State represents the current state of a workflow execution
type State struct {
	ID          string         `json:"id"`
	WorkflowID  string         `json:"workflow_id"`
	Status      Status         `json:"status"`
	CurrentStep string         `json:"current_step"`
	StepStates  map[string]any `json:"step_states"`
	Context     map[string]any `json:"context"`
	StartedAt   time.Time      `json:"started_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
	Error       string         `json:"error,omitempty"`
	Checkpoints []Checkpoint   `json:"checkpoints,omitempty"`
}

// Status represents workflow execution status
type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusPaused    Status = "paused"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

// Checkpoint represents a restorable point in workflow execution
type Checkpoint struct {
	ID        string         `json:"id"`
	StepID    string         `json:"step_id"`
	State     map[string]any `json:"state"`
	CreatedAt time.Time      `json:"created_at"`
}

// Store defines the interface for workflow state persistence
type Store interface {
	// Save persists the current workflow state
	Save(state *State) error

	// Load retrieves a workflow state by execution ID
	Load(executionID string) (*State, error)

	// List returns all workflow states, optionally filtered by workflow ID
	List(workflowID string) ([]*State, error)

	// Delete removes a workflow state
	Delete(executionID string) error

	// SaveCheckpoint saves a checkpoint for recovery
	SaveCheckpoint(executionID string, checkpoint *Checkpoint) error

	// LoadLatestCheckpoint loads the most recent checkpoint
	LoadLatestCheckpoint(executionID string) (*Checkpoint, error)
}

// FileStore implements Store using the filesystem
type FileStore struct {
	baseDir string
	mu      sync.RWMutex
}

// NewFileStore creates a new file-based workflow store
func NewFileStore(baseDir string) (*FileStore, error) {
	if err := os.MkdirAll(baseDir, 0700); err != nil {
		return nil, fmt.Errorf("create store directory: %w", err)
	}

	checkpointDir := filepath.Join(baseDir, "checkpoints")
	if err := os.MkdirAll(checkpointDir, 0700); err != nil {
		return nil, fmt.Errorf("create checkpoints directory: %w", err)
	}

	return &FileStore{baseDir: baseDir}, nil
}

// Save persists workflow state to a file
func (s *FileStore) Save(state *State) error {
	if err := validateID(state.ID); err != nil {
		return fmt.Errorf("invalid execution ID: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	state.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	filePath := s.statePath(state.ID)
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("write state file: %w", err)
	}

	return nil
}

// Load retrieves workflow state from a file
func (s *FileStore) Load(executionID string) (*State, error) {
	if err := validateID(executionID); err != nil {
		return nil, fmt.Errorf("invalid execution ID: %w", err)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	filePath := s.statePath(executionID)
	// G304: Path is constructed from validated executionID via statePath() which uses filepath.Join
	data, err := os.ReadFile(filePath) //nolint:gosec
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("workflow state not found: %s", executionID)
		}
		return nil, fmt.Errorf("read state file: %w", err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("unmarshal state: %w", err)
	}

	return &state, nil
}

// List returns all workflow states
func (s *FileStore) List(workflowID string) ([]*State, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}

	var states []*State
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		filePath := filepath.Join(s.baseDir, entry.Name())
		// G304: Path is constructed from trusted baseDir and validated entry.Name() from os.ReadDir
		data, err := os.ReadFile(filePath) //nolint:gosec
		if err != nil {
			continue
		}

		var state State
		if err := json.Unmarshal(data, &state); err != nil {
			continue
		}

		if workflowID == "" || state.WorkflowID == workflowID {
			states = append(states, &state)
		}
	}

	return states, nil
}

// Delete removes a workflow state file
func (s *FileStore) Delete(executionID string) error {
	if err := validateID(executionID); err != nil {
		return fmt.Errorf("invalid execution ID: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	filePath := s.statePath(executionID)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove state file: %w", err)
	}

	// Also remove checkpoints
	checkpointDir := filepath.Join(s.baseDir, "checkpoints", executionID)
	if err := os.RemoveAll(checkpointDir); err != nil {
		return fmt.Errorf("remove checkpoints: %w", err)
	}

	return nil
}

// SaveCheckpoint saves a checkpoint for recovery
func (s *FileStore) SaveCheckpoint(executionID string, checkpoint *Checkpoint) error {
	if err := validateID(executionID); err != nil {
		return fmt.Errorf("invalid execution ID: %w", err)
	}
	if err := validateID(checkpoint.ID); err != nil {
		return fmt.Errorf("invalid checkpoint ID: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	checkpoint.CreatedAt = time.Now()

	checkpointDir := filepath.Join(s.baseDir, "checkpoints", executionID)
	if err := os.MkdirAll(checkpointDir, 0700); err != nil {
		return fmt.Errorf("create checkpoint directory: %w", err)
	}

	data, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal checkpoint: %w", err)
	}

	filePath := filepath.Join(checkpointDir, checkpoint.ID+".json")
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("write checkpoint file: %w", err)
	}

	return nil
}

// LoadLatestCheckpoint loads the most recent checkpoint
func (s *FileStore) LoadLatestCheckpoint(executionID string) (*Checkpoint, error) {
	if err := validateID(executionID); err != nil {
		return nil, fmt.Errorf("invalid execution ID: %w", err)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	checkpointDir := filepath.Join(s.baseDir, "checkpoints", executionID)
	entries, err := os.ReadDir(checkpointDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read checkpoint directory: %w", err)
	}

	var latest *Checkpoint
	var latestTime time.Time

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		filePath := filepath.Join(checkpointDir, entry.Name())
		// G304: Path is constructed from trusted checkpointDir and validated entry.Name() from os.ReadDir
		data, err := os.ReadFile(filePath) //nolint:gosec
		if err != nil {
			continue
		}

		var checkpoint Checkpoint
		if err := json.Unmarshal(data, &checkpoint); err != nil {
			continue
		}

		if latest == nil || checkpoint.CreatedAt.After(latestTime) {
			latest = &checkpoint
			latestTime = checkpoint.CreatedAt
		}
	}

	return latest, nil
}

func (s *FileStore) statePath(executionID string) string {
	return filepath.Join(s.baseDir, executionID+".json")
}

// MemoryStore implements Store using in-memory storage (useful for testing)
type MemoryStore struct {
	states      map[string]*State
	checkpoints map[string][]*Checkpoint
	mu          sync.RWMutex
}

// NewMemoryStore creates a new in-memory workflow store
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		states:      make(map[string]*State),
		checkpoints: make(map[string][]*Checkpoint),
	}
}

// Save persists workflow state in memory
func (s *MemoryStore) Save(state *State) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	state.UpdatedAt = time.Now()
	// Deep copy to prevent external mutations
	data, _ := json.Marshal(state)
	var copy State
	_ = json.Unmarshal(data, &copy)
	s.states[state.ID] = &copy

	return nil
}

// Load retrieves workflow state from memory
func (s *MemoryStore) Load(executionID string) (*State, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state, exists := s.states[executionID]
	if !exists {
		return nil, fmt.Errorf("workflow state not found: %s", executionID)
	}

	// Deep copy to prevent external mutations
	data, _ := json.Marshal(state)
	var copy State
	_ = json.Unmarshal(data, &copy)

	return &copy, nil
}

// List returns all workflow states from memory
func (s *MemoryStore) List(workflowID string) ([]*State, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var states []*State
	for _, state := range s.states {
		if workflowID == "" || state.WorkflowID == workflowID {
			// Deep copy
			data, _ := json.Marshal(state)
			var copy State
			_ = json.Unmarshal(data, &copy)
			states = append(states, &copy)
		}
	}

	return states, nil
}

// Delete removes workflow state from memory
func (s *MemoryStore) Delete(executionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.states, executionID)
	delete(s.checkpoints, executionID)

	return nil
}

// SaveCheckpoint saves a checkpoint in memory
func (s *MemoryStore) SaveCheckpoint(executionID string, checkpoint *Checkpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	checkpoint.CreatedAt = time.Now()
	s.checkpoints[executionID] = append(s.checkpoints[executionID], checkpoint)

	return nil
}

// LoadLatestCheckpoint loads the most recent checkpoint from memory
func (s *MemoryStore) LoadLatestCheckpoint(executionID string) (*Checkpoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	checkpoints := s.checkpoints[executionID]
	if len(checkpoints) == 0 {
		return nil, nil
	}

	return checkpoints[len(checkpoints)-1], nil
}
