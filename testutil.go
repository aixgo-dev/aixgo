package aixgo

import (
	"fmt"
	"sync"
)

// MockFileReader is a mock implementation of FileReader for testing
type MockFileReader struct {
	files map[string][]byte
	err   error
	mu    sync.RWMutex
}

// NewMockFileReader creates a new mock file reader
func NewMockFileReader() *MockFileReader {
	return &MockFileReader{
		files: make(map[string][]byte),
	}
}

// ReadFile implements FileReader.ReadFile
func (m *MockFileReader) ReadFile(path string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.err != nil {
		return nil, m.err
	}

	data, ok := m.files[path]
	if !ok {
		return nil, fmt.Errorf("file not found: %s", path)
	}

	return data, nil
}

// AddFile adds a file to the mock file system
func (m *MockFileReader) AddFile(path string, content []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[path] = content
}

// SetError sets an error to return from ReadFile
func (m *MockFileReader) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

// Reset clears all files and errors
func (m *MockFileReader) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files = make(map[string][]byte)
	m.err = nil
}
