package memory

import (
	"sync"
	"time"
)

// Config holds configuration for semantic memory
type Config struct {
	MaxMemories         int     // Maximum number of memories to retain
	SimilarityThreshold float64 // Minimum similarity score for retrieval
}

// Memory represents a stored piece of information
type Memory struct {
	ID        string
	Content   string
	Embedding []float64
	Metadata  map[string]any
	Timestamp time.Time
}

// SemanticMemory provides semantic storage and retrieval of information
type SemanticMemory struct {
	config   Config
	memories []Memory
	mu       sync.RWMutex
}

// NewSemanticMemory creates a new semantic memory instance
func NewSemanticMemory(cfg Config) *SemanticMemory {
	if cfg.MaxMemories <= 0 {
		cfg.MaxMemories = 100
	}
	if cfg.SimilarityThreshold <= 0 {
		cfg.SimilarityThreshold = 0.7
	}

	return &SemanticMemory{
		config:   cfg,
		memories: make([]Memory, 0, cfg.MaxMemories),
	}
}

// Store adds a memory to the semantic memory
func (sm *SemanticMemory) Store(content string, embedding []float64, metadata map[string]any) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	mem := Memory{
		ID:        generateID(),
		Content:   content,
		Embedding: embedding,
		Metadata:  metadata,
		Timestamp: time.Now(),
	}

	sm.memories = append(sm.memories, mem)

	// Evict oldest if over capacity
	if len(sm.memories) > sm.config.MaxMemories {
		sm.memories = sm.memories[1:]
	}

	return nil
}

// Retrieve finds memories similar to the given embedding
func (sm *SemanticMemory) Retrieve(embedding []float64, limit int) ([]Memory, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	type scoredMemory struct {
		memory Memory
		score  float64
	}

	scored := make([]scoredMemory, 0, len(sm.memories))
	for _, mem := range sm.memories {
		similarity := cosineSimilarity(embedding, mem.Embedding)
		if similarity >= sm.config.SimilarityThreshold {
			scored = append(scored, scoredMemory{mem, similarity})
		}
	}

	// Sort by score descending
	for i := 0; i < len(scored)-1; i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// Return top N
	if limit > len(scored) {
		limit = len(scored)
	}

	results := make([]Memory, limit)
	for i := 0; i < limit; i++ {
		results[i] = scored[i].memory
	}

	return results, nil
}

// Clear removes all memories
func (sm *SemanticMemory) Clear() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.memories = make([]Memory, 0, sm.config.MaxMemories)
}

// Count returns the number of stored memories
func (sm *SemanticMemory) Count() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.memories)
}

// cosineSimilarity computes cosine similarity between two embeddings
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0.0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0.0
	}

	return dotProduct / (sqrt(normA) * sqrt(normB))
}

func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}

	// Newton's method for square root
	z := x
	for i := 0; i < 10; i++ {
		z -= (z*z - x) / (2 * z)
	}
	return z
}

var idCounter uint64
var idMutex sync.Mutex

func generateID() string {
	idMutex.Lock()
	defer idMutex.Unlock()
	idCounter++
	return time.Now().Format("20060102150405") + "-" + itoa(idCounter)
}

func itoa(n uint64) string {
	if n == 0 {
		return "0"
	}

	var buf [20]byte
	i := len(buf) - 1
	for n > 0 {
		buf[i] = byte('0' + n%10)
		n /= 10
		i--
	}
	return string(buf[i+1:])
}
