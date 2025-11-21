package workflow

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMemoryStore(t *testing.T) {
	store := NewMemoryStore()

	// Test Save and Load
	state := &State{
		ID:          "exec-1",
		WorkflowID:  "workflow-1",
		Status:      StatusRunning,
		CurrentStep: "step-1",
		StepStates:  map[string]any{"step-0": "done"},
		Context:     map[string]any{"input": "test"},
		StartedAt:   time.Now(),
	}

	if err := store.Save(state); err != nil {
		t.Fatalf("save error: %v", err)
	}

	loaded, err := store.Load("exec-1")
	if err != nil {
		t.Fatalf("load error: %v", err)
	}

	if loaded.ID != state.ID {
		t.Errorf("expected ID %q, got %q", state.ID, loaded.ID)
	}
	if loaded.Status != state.Status {
		t.Errorf("expected status %q, got %q", state.Status, loaded.Status)
	}

	// Test List
	state2 := &State{
		ID:         "exec-2",
		WorkflowID: "workflow-1",
		Status:     StatusCompleted,
		StartedAt:  time.Now(),
	}
	_ = store.Save(state2)

	states, err := store.List("workflow-1")
	if err != nil {
		t.Fatalf("list error: %v", err)
	}
	if len(states) != 2 {
		t.Errorf("expected 2 states, got %d", len(states))
	}

	// Test Delete
	if err := store.Delete("exec-1"); err != nil {
		t.Fatalf("delete error: %v", err)
	}

	_, err = store.Load("exec-1")
	if err == nil {
		t.Error("expected error loading deleted state")
	}
}

func TestMemoryStore_Checkpoints(t *testing.T) {
	store := NewMemoryStore()

	checkpoint1 := &Checkpoint{
		ID:     "cp-1",
		StepID: "step-1",
		State:  map[string]any{"data": "first"},
	}
	checkpoint2 := &Checkpoint{
		ID:     "cp-2",
		StepID: "step-2",
		State:  map[string]any{"data": "second"},
	}

	if err := store.SaveCheckpoint("exec-1", checkpoint1); err != nil {
		t.Fatalf("save checkpoint 1: %v", err)
	}
	if err := store.SaveCheckpoint("exec-1", checkpoint2); err != nil {
		t.Fatalf("save checkpoint 2: %v", err)
	}

	latest, err := store.LoadLatestCheckpoint("exec-1")
	if err != nil {
		t.Fatalf("load latest checkpoint: %v", err)
	}

	if latest.ID != "cp-2" {
		t.Errorf("expected latest checkpoint cp-2, got %q", latest.ID)
	}
}

func TestFileStore(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "workflow-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	// Test Save and Load
	state := &State{
		ID:          "exec-1",
		WorkflowID:  "workflow-1",
		Status:      StatusRunning,
		CurrentStep: "step-1",
		StepStates:  map[string]any{"step-0": "done"},
		Context:     map[string]any{"input": "test"},
		StartedAt:   time.Now(),
	}

	if err := store.Save(state); err != nil {
		t.Fatalf("save error: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(filepath.Join(tmpDir, "exec-1.json")); err != nil {
		t.Errorf("state file not created: %v", err)
	}

	loaded, err := store.Load("exec-1")
	if err != nil {
		t.Fatalf("load error: %v", err)
	}

	if loaded.ID != state.ID {
		t.Errorf("expected ID %q, got %q", state.ID, loaded.ID)
	}

	// Test List
	state2 := &State{
		ID:         "exec-2",
		WorkflowID: "workflow-1",
		Status:     StatusCompleted,
		StartedAt:  time.Now(),
	}
	_ = store.Save(state2)

	states, err := store.List("workflow-1")
	if err != nil {
		t.Fatalf("list error: %v", err)
	}
	if len(states) != 2 {
		t.Errorf("expected 2 states, got %d", len(states))
	}

	// Test Delete
	if err := store.Delete("exec-1"); err != nil {
		t.Fatalf("delete error: %v", err)
	}

	_, err = store.Load("exec-1")
	if err == nil {
		t.Error("expected error loading deleted state")
	}
}

func TestFileStore_Checkpoints(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "workflow-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	checkpoint1 := &Checkpoint{
		ID:     "cp-1",
		StepID: "step-1",
		State:  map[string]any{"data": "first"},
	}

	if err := store.SaveCheckpoint("exec-1", checkpoint1); err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}

	// Wait a bit to ensure different timestamps
	time.Sleep(10 * time.Millisecond)

	checkpoint2 := &Checkpoint{
		ID:     "cp-2",
		StepID: "step-2",
		State:  map[string]any{"data": "second"},
	}

	if err := store.SaveCheckpoint("exec-1", checkpoint2); err != nil {
		t.Fatalf("save checkpoint 2: %v", err)
	}

	latest, err := store.LoadLatestCheckpoint("exec-1")
	if err != nil {
		t.Fatalf("load latest checkpoint: %v", err)
	}

	if latest.ID != "cp-2" {
		t.Errorf("expected latest checkpoint cp-2, got %q", latest.ID)
	}
}

func TestStatus(t *testing.T) {
	tests := []struct {
		status Status
		value  string
	}{
		{StatusPending, "pending"},
		{StatusRunning, "running"},
		{StatusPaused, "paused"},
		{StatusCompleted, "completed"},
		{StatusFailed, "failed"},
		{StatusCancelled, "cancelled"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.value {
			t.Errorf("expected %q, got %q", tt.value, tt.status)
		}
	}
}

func TestValidateID_PathTraversal(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{"valid_simple", "exec-123", false},
		{"valid_underscore", "exec_123_abc", false},
		{"valid_uuid", "550e8400-e29b-41d4-a716-446655440000", false},
		{"path_traversal_parent", "../etc/passwd", true},
		{"path_traversal_absolute", "/etc/passwd", true},
		{"path_traversal_hidden", "..%2F..%2Fetc", true},
		{"empty", "", true},
		{"with_slash", "exec/123", true},
		{"with_backslash", "exec\\123", true},
		{"with_dot_dot", "exec..123", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateID(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateID(%q) error = %v, wantErr %v", tt.id, err, tt.wantErr)
			}
		})
	}
}

func TestFileStore_PathTraversalPrevention(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "workflow-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	// Test Load with path traversal attempt
	_, err = store.Load("../../../etc/passwd")
	if err == nil {
		t.Error("expected error for path traversal in Load")
	}

	// Test Delete with path traversal attempt
	err = store.Delete("../../../etc/passwd")
	if err == nil {
		t.Error("expected error for path traversal in Delete")
	}

	// Test SaveCheckpoint with path traversal in execution ID
	checkpoint := &Checkpoint{ID: "cp-1", StepID: "step-1"}
	err = store.SaveCheckpoint("../../../etc", checkpoint)
	if err == nil {
		t.Error("expected error for path traversal in SaveCheckpoint execution ID")
	}

	// Test SaveCheckpoint with path traversal in checkpoint ID
	checkpoint2 := &Checkpoint{ID: "../../../etc/passwd", StepID: "step-1"}
	err = store.SaveCheckpoint("exec-1", checkpoint2)
	if err == nil {
		t.Error("expected error for path traversal in SaveCheckpoint checkpoint ID")
	}

	// Test LoadLatestCheckpoint with path traversal
	_, err = store.LoadLatestCheckpoint("../../../etc")
	if err == nil {
		t.Error("expected error for path traversal in LoadLatestCheckpoint")
	}

	// Test Save with path traversal in state ID
	state := &State{ID: "../../../etc/passwd", WorkflowID: "wf-1", Status: StatusRunning}
	err = store.Save(state)
	if err == nil {
		t.Error("expected error for path traversal in Save")
	}
}
