package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aixgo-dev/aixgo/pkg/vectorstore"
)

// MemoryVectorStore implements an in-memory vector store for testing and development.
// It uses brute-force search and is suitable for small to medium datasets.
//
// Features:
//   - Collection-based isolation
//   - Built-in TTL cleanup
//   - Content hash deduplication
//   - Indexed lookups for scope/temporal fields
//   - Thread-safe operations
//   - Streaming query support
type MemoryVectorStore struct {
	collections map[string]*MemoryCollection
	mu          sync.RWMutex
	stopCleanup chan struct{}
	wg          sync.WaitGroup
}

// New creates a new MemoryVectorStore.
func New() (vectorstore.VectorStore, error) {
	store := &MemoryVectorStore{
		collections: make(map[string]*MemoryCollection),
		stopCleanup: make(chan struct{}),
	}

	// Start TTL cleanup goroutine
	store.wg.Add(1)
	go store.cleanupExpiredDocuments()

	return store, nil
}

// Collection returns a collection with the specified name and options.
func (m *MemoryVectorStore) Collection(name string, opts ...vectorstore.CollectionOption) vectorstore.Collection {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Return existing collection if already created
	if coll, exists := m.collections[name]; exists {
		return coll
	}

	// Create new collection
	config := vectorstore.ApplyOptions(opts)
	coll := &MemoryCollection{
		name:       name,
		config:     config,
		documents:  make(map[string]*vectorstore.Document),
		scopeIndex: newScopeIndex(),
		timeIndex:  newTimeIndex(),
		tagIndex:   newTagIndex(),
		hashIndex:  make(map[string]string),
		createdAt:  time.Now(),
		updatedAt:  time.Now(),
	}

	m.collections[name] = coll
	return coll
}

// ListCollections returns the names of all collections.
func (m *MemoryVectorStore) ListCollections(ctx context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.collections))
	for name := range m.collections {
		names = append(names, name)
	}

	sort.Strings(names)
	return names, nil
}

// DeleteCollection permanently deletes a collection and all its documents.
func (m *MemoryVectorStore) DeleteCollection(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.collections[name]; !exists {
		return fmt.Errorf("collection %q does not exist", name)
	}

	delete(m.collections, name)
	return nil
}

// Stats returns statistics about the vector store.
func (m *MemoryVectorStore) Stats(ctx context.Context) (*vectorstore.StoreStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var totalDocs int64
	var totalBytes int64

	for _, coll := range m.collections {
		coll.mu.RLock()
		totalDocs += int64(len(coll.documents))
		// Estimate storage bytes (rough approximation)
		for _, doc := range coll.documents {
			totalBytes += estimateDocumentSize(doc)
		}
		coll.mu.RUnlock()
	}

	return &vectorstore.StoreStats{
		Collections:  int64(len(m.collections)),
		Documents:    totalDocs,
		StorageBytes: totalBytes,
		Provider:     "memory",
		Version:      "1.0.0",
	}, nil
}

// Close closes the vector store and releases resources.
func (m *MemoryVectorStore) Close() error {
	close(m.stopCleanup)
	m.wg.Wait()
	return nil
}

// cleanupExpiredDocuments runs periodically to remove expired documents.
func (m *MemoryVectorStore) cleanupExpiredDocuments() {
	defer m.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.mu.RLock()
			collections := make([]*MemoryCollection, 0, len(m.collections))
			for _, coll := range m.collections {
				collections = append(collections, coll)
			}
			m.mu.RUnlock()

			// Clean up each collection
			for _, coll := range collections {
				coll.cleanupExpired()
			}

		case <-m.stopCleanup:
			return
		}
	}
}

// MemoryCollection represents an isolated collection of documents.
type MemoryCollection struct {
	name       string
	config     *vectorstore.CollectionConfig
	documents  map[string]*vectorstore.Document
	scopeIndex *scopeIndex
	timeIndex  *timeIndex
	tagIndex   *tagIndex
	hashIndex  map[string]string // content hash -> document ID
	createdAt  time.Time
	updatedAt  time.Time
	mu         sync.RWMutex
}

// Name returns the collection name.
func (c *MemoryCollection) Name() string {
	return c.name
}

// Upsert inserts or updates documents in the collection.
func (c *MemoryCollection) Upsert(ctx context.Context, documents ...*vectorstore.Document) (*vectorstore.UpsertResult, error) {
	if len(documents) == 0 {
		return &vectorstore.UpsertResult{}, nil
	}

	startTime := time.Now()

	// Validate all documents first
	validationStart := time.Now()
	for i, doc := range documents {
		if err := vectorstore.Validate(doc); err != nil {
			return nil, fmt.Errorf("invalid document at index %d: %w", i, err)
		}

		// Validate embedding dimensions if configured
		if c.config.EmbeddingDimensions > 0 && doc.Embedding != nil {
			if doc.Embedding.Dimensions != c.config.EmbeddingDimensions {
				return nil, fmt.Errorf("document %s embedding dimension mismatch: expected %d, got %d",
					doc.ID, c.config.EmbeddingDimensions, doc.Embedding.Dimensions)
			}
		}

		// Validate required scope fields
		if err := c.validateRequiredScope(doc); err != nil {
			return nil, fmt.Errorf("document %s: %w", doc.ID, err)
		}
	}
	validationTime := time.Since(validationStart)

	c.mu.Lock()
	defer c.mu.Unlock()

	storageStart := time.Now()
	result := &vectorstore.UpsertResult{}

	for _, doc := range documents {
		// Check deduplication
		if c.config.EnableDeduplication {
			contentHash := calculateContentHash(doc)
			if existingID, exists := c.hashIndex[contentHash]; exists && existingID != doc.ID {
				// Check if similarity exceeds threshold
				if existingDoc, ok := c.documents[existingID]; ok {
					if doc.Embedding != nil && existingDoc.Embedding != nil {
						similarity := cosineSimilarity(doc.Embedding.Vector, existingDoc.Embedding.Vector)
						if similarity >= c.config.DeduplicationThreshold {
							result.Deduplicated++
							result.DeduplicatedIDs = append(result.DeduplicatedIDs, doc.ID)
							continue
						}
					}
				}
			}
		}

		// Check if document exists
		_, exists := c.documents[doc.ID]

		// Set temporal information
		now := time.Now()
		if doc.Temporal == nil {
			doc.Temporal = &vectorstore.Temporal{
				CreatedAt: now,
				UpdatedAt: now,
			}
		} else {
			if doc.Temporal.CreatedAt.IsZero() {
				doc.Temporal.CreatedAt = now
			}
			doc.Temporal.UpdatedAt = now
		}

		// Apply TTL if configured
		if c.config.TTL > 0 && doc.Temporal.ExpiresAt == nil {
			expiresAt := now.Add(c.config.TTL)
			doc.Temporal.ExpiresAt = &expiresAt
		}

		// Deep copy document to prevent external mutation
		docCopy := deepCopyDocument(doc)

		// Store document
		c.documents[doc.ID] = docCopy

		// Update indexes
		c.scopeIndex.add(doc.ID, doc.Scope)
		c.timeIndex.add(doc.ID, doc.Temporal)
		c.tagIndex.add(doc.ID, doc.Tags)
		if c.config.EnableDeduplication {
			contentHash := calculateContentHash(doc)
			c.hashIndex[contentHash] = doc.ID
		}

		if exists {
			result.Updated++
		} else {
			result.Inserted++
		}
	}

	storageTime := time.Since(storageStart)

	c.updatedAt = time.Now()

	result.Timing = &vectorstore.OperationTiming{
		Total:      time.Since(startTime),
		Validation: validationTime,
		Storage:    storageTime,
	}

	return result, nil
}

// UpsertBatch performs batch upsert with progress tracking.
func (c *MemoryCollection) UpsertBatch(ctx context.Context, documents []*vectorstore.Document, opts ...vectorstore.BatchOption) (*vectorstore.UpsertResult, error) {
	if len(documents) == 0 {
		return &vectorstore.UpsertResult{}, nil
	}

	config := vectorstore.ApplyBatchOptions(opts)

	// Validate before batch if requested
	if config.ValidateBeforeBatch {
		for i, doc := range documents {
			if err := vectorstore.Validate(doc); err != nil {
				return nil, fmt.Errorf("invalid document at index %d: %w", i, err)
			}
		}
	}

	totalResult := &vectorstore.UpsertResult{}
	processed := 0

	// Process in batches
	for i := 0; i < len(documents); i += config.BatchSize {
		end := i + config.BatchSize
		if end > len(documents) {
			end = len(documents)
		}

		batch := documents[i:end]
		batchResult, err := c.Upsert(ctx, batch...)

		if err != nil {
			if !config.ContinueOnError {
				return totalResult, err
			}
			totalResult.Failed += int64(len(batch))
		} else {
			totalResult.Inserted += batchResult.Inserted
			totalResult.Updated += batchResult.Updated
			totalResult.Deduplicated += batchResult.Deduplicated
			totalResult.DeduplicatedIDs = append(totalResult.DeduplicatedIDs, batchResult.DeduplicatedIDs...)
		}

		processed += len(batch)

		// Call progress callback
		if config.ProgressCallback != nil {
			config.ProgressCallback(processed, len(documents))
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return totalResult, ctx.Err()
		default:
		}
	}

	return totalResult, nil
}

// Query performs similarity search and returns matching documents.
func (c *MemoryCollection) Query(ctx context.Context, query *vectorstore.Query) (*vectorstore.QueryResult, error) {
	if err := query.Validate(); err != nil {
		return nil, fmt.Errorf("invalid query: %w", err)
	}

	startTime := time.Now()
	timing := &vectorstore.QueryTiming{}

	c.mu.RLock()
	defer c.mu.RUnlock()

	// Apply filters first
	filterStart := time.Now()
	candidates := c.applyFilters(query.Filters)
	timing.FilterApplication = time.Since(filterStart)

	// Calculate similarity scores if embedding provided
	var matches []*vectorstore.Match
	if query.Embedding != nil {
		scoringStart := time.Now()
		matches = c.calculateScores(candidates, query)
		timing.Scoring = time.Since(scoringStart)
	} else {
		// Filter-only query
		matches = make([]*vectorstore.Match, 0, len(candidates))
		for _, docID := range candidates {
			if doc, exists := c.documents[docID]; exists {
				matches = append(matches, &vectorstore.Match{
					Document: doc,
					Score:    1.0,
				})
			}
		}
	}

	// Apply offset and limit
	total := int64(len(matches))
	if query.Offset > 0 && query.Offset < len(matches) {
		matches = matches[query.Offset:]
	} else if query.Offset >= len(matches) {
		matches = nil
	}

	if query.Limit > 0 && query.Limit < len(matches) {
		matches = matches[:query.Limit]
	}

	// Set ranks
	for i, match := range matches {
		match.Rank = i + 1 + query.Offset
	}

	timing.Total = time.Since(startTime)

	result := &vectorstore.QueryResult{
		Matches: matches,
		Total:   total,
		Offset:  query.Offset,
		Limit:   query.Limit,
		Timing:  timing,
	}

	return result, nil
}

// QueryStream performs similarity search and streams results via an iterator.
func (c *MemoryCollection) QueryStream(ctx context.Context, query *vectorstore.Query) (vectorstore.ResultIterator, error) {
	// For memory store, we materialize all results and return a slice iterator
	result, err := c.Query(ctx, query)
	if err != nil {
		return vectorstore.NewErrorIterator(err), nil
	}

	return vectorstore.NewSliceIterator(result.Matches), nil
}

// Get retrieves documents by their IDs.
func (c *MemoryCollection) Get(ctx context.Context, ids ...string) ([]*vectorstore.Document, error) {
	if len(ids) == 0 {
		return []*vectorstore.Document{}, nil
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	documents := make([]*vectorstore.Document, 0, len(ids))
	for _, id := range ids {
		if doc, exists := c.documents[id]; exists {
			// Deep copy to prevent external mutation
			docCopy := deepCopyDocument(doc)
			documents = append(documents, docCopy)
		}
	}

	return documents, nil
}

// Delete removes documents by their IDs.
func (c *MemoryCollection) Delete(ctx context.Context, ids ...string) (*vectorstore.DeleteResult, error) {
	if len(ids) == 0 {
		return &vectorstore.DeleteResult{}, nil
	}

	startTime := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	result := &vectorstore.DeleteResult{}

	for _, id := range ids {
		doc, exists := c.documents[id]
		if !exists {
			result.NotFound++
			result.NotFoundIDs = append(result.NotFoundIDs, id)
			continue
		}

		// Remove from indexes
		c.scopeIndex.remove(id)
		c.timeIndex.remove(id)
		c.tagIndex.remove(id)
		if c.config.EnableDeduplication {
			contentHash := calculateContentHash(doc)
			delete(c.hashIndex, contentHash)
		}

		delete(c.documents, id)
		result.Deleted++
	}

	c.updatedAt = time.Now()

	result.Timing = &vectorstore.OperationTiming{
		Total: time.Since(startTime),
	}

	return result, nil
}

// DeleteByFilter removes all documents matching the filter.
func (c *MemoryCollection) DeleteByFilter(ctx context.Context, filter vectorstore.Filter) (*vectorstore.DeleteResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Find matching documents
	candidates := c.applyFilters(filter)

	result := &vectorstore.DeleteResult{}
	startTime := time.Now()

	for _, id := range candidates {
		doc, exists := c.documents[id]
		if !exists {
			continue
		}

		// Remove from indexes
		c.scopeIndex.remove(id)
		c.timeIndex.remove(id)
		c.tagIndex.remove(id)
		if c.config.EnableDeduplication {
			contentHash := calculateContentHash(doc)
			delete(c.hashIndex, contentHash)
		}

		delete(c.documents, id)
		result.Deleted++
	}

	c.updatedAt = time.Now()

	result.Timing = &vectorstore.OperationTiming{
		Total: time.Since(startTime),
	}

	return result, nil
}

// Count returns the number of documents in the collection.
func (c *MemoryCollection) Count(ctx context.Context, filter vectorstore.Filter) (int64, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if filter == nil {
		return int64(len(c.documents)), nil
	}

	candidates := c.applyFilters(filter)
	return int64(len(candidates)), nil
}

// Stats returns statistics about the collection.
func (c *MemoryCollection) Stats(ctx context.Context) (*vectorstore.CollectionStats, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var totalBytes int64
	var embeddingDims int

	for _, doc := range c.documents {
		totalBytes += estimateDocumentSize(doc)
		if doc.Embedding != nil && embeddingDims == 0 {
			embeddingDims = doc.Embedding.Dimensions
		}
	}

	return &vectorstore.CollectionStats{
		Name:                c.name,
		Documents:           int64(len(c.documents)),
		StorageBytes:        totalBytes,
		EmbeddingDimensions: embeddingDims,
		IndexType:           string(c.config.IndexType),
		CreatedAt:           vectorstore.NewTimestamp(c.createdAt),
		UpdatedAt:           vectorstore.NewTimestamp(c.updatedAt),
	}, nil
}

// Clear removes all documents from the collection.
func (c *MemoryCollection) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.documents = make(map[string]*vectorstore.Document)
	c.scopeIndex = newScopeIndex()
	c.timeIndex = newTimeIndex()
	c.tagIndex = newTagIndex()
	c.hashIndex = make(map[string]string)
	c.updatedAt = time.Now()

	return nil
}

// Helper methods

// validateRequiredScope validates that document has required scope fields.
func (c *MemoryCollection) validateRequiredScope(doc *vectorstore.Document) error {
	if len(c.config.ScopeRequired) == 0 {
		return nil
	}

	if doc.Scope == nil {
		return fmt.Errorf("scope is required but not provided")
	}

	for _, field := range c.config.ScopeRequired {
		switch field {
		case "tenant":
			if doc.Scope.Tenant == "" {
				return fmt.Errorf("required scope field 'tenant' is empty")
			}
		case "user":
			if doc.Scope.User == "" {
				return fmt.Errorf("required scope field 'user' is empty")
			}
		case "session":
			if doc.Scope.Session == "" {
				return fmt.Errorf("required scope field 'session' is empty")
			}
		case "agent":
			if doc.Scope.Agent == "" {
				return fmt.Errorf("required scope field 'agent' is empty")
			}
		case "thread":
			if doc.Scope.Thread == "" {
				return fmt.Errorf("required scope field 'thread' is empty")
			}
		}
	}

	return nil
}

// applyFilters applies filters and returns matching document IDs.
func (c *MemoryCollection) applyFilters(filter vectorstore.Filter) []string {
	if filter == nil {
		// No filter - return all document IDs
		ids := make([]string, 0, len(c.documents))
		for id := range c.documents {
			ids = append(ids, id)
		}
		return ids
	}

	// Get all candidates first
	candidates := make(map[string]bool)
	for id := range c.documents {
		candidates[id] = true
	}

	// Apply filter
	for id := range candidates {
		doc := c.documents[id]
		if !c.matchesFilter(doc, filter) {
			delete(candidates, id)
		}
	}

	// Convert to slice
	ids := make([]string, 0, len(candidates))
	for id := range candidates {
		ids = append(ids, id)
	}

	return ids
}

// matchesFilter checks if a document matches a filter.
func (c *MemoryCollection) matchesFilter(doc *vectorstore.Document, filter vectorstore.Filter) bool {
	if filter == nil {
		return true
	}

	// Handle composite filters
	if vectorstore.IsAndFilter(filter) {
		filters := vectorstore.GetFilters(filter)
		for _, f := range filters {
			if !c.matchesFilter(doc, f) {
				return false
			}
		}
		return true
	}

	if vectorstore.IsOrFilter(filter) {
		filters := vectorstore.GetFilters(filter)
		for _, f := range filters {
			if c.matchesFilter(doc, f) {
				return true
			}
		}
		return false
	}

	if vectorstore.IsNotFilter(filter) {
		inner, _ := vectorstore.GetNotFilter(filter)
		return !c.matchesFilter(doc, inner)
	}

	// Handle field filters
	if field, op, value, ok := vectorstore.GetFieldFilter(filter); ok {
		return c.matchesFieldFilter(doc, field, op, value)
	}

	// Handle tag filters
	if tag, ok := vectorstore.GetTagFilter(filter); ok {
		return c.matchesTagFilter(doc, tag)
	}

	// Handle scope filters
	if scope, ok := vectorstore.GetScopeFilter(filter); ok {
		return c.matchesScopeFilter(doc, scope)
	}

	// Handle time filters
	if field, op, value, ok := vectorstore.GetTimeFilter(filter); ok {
		return c.matchesTimeFilter(doc, field, op, value)
	}

	// Handle score filters (not applicable during filtering, only during scoring)
	if _, _, ok := vectorstore.GetScoreFilter(filter); ok {
		return true // Score filters are applied during scoring
	}

	return true
}

// matchesFieldFilter checks if document matches a field filter.
func (c *MemoryCollection) matchesFieldFilter(doc *vectorstore.Document, field string, op vectorstore.FilterOperator, value any) bool {
	docValue, exists := doc.Metadata[field]

	switch op {
	case vectorstore.OpExists:
		return exists
	case vectorstore.OpNotExists:
		return !exists
	case vectorstore.OpEqual:
		return exists && docValue == value
	case vectorstore.OpNotEqual:
		return !exists || docValue != value
	case vectorstore.OpIn:
		if !exists {
			return false
		}
		values, ok := value.([]any)
		if !ok {
			return false
		}
		for _, v := range values {
			if docValue == v {
				return true
			}
		}
		return false
	case vectorstore.OpNotIn:
		if !exists {
			return true
		}
		values, ok := value.([]any)
		if !ok {
			return true
		}
		for _, v := range values {
			if docValue == v {
				return false
			}
		}
		return true
	case vectorstore.OpContains:
		if !exists {
			return false
		}
		str, ok := docValue.(string)
		if !ok {
			return false
		}
		substr, ok := value.(string)
		if !ok {
			return false
		}
		return strings.Contains(str, substr)
	case vectorstore.OpStartsWith:
		if !exists {
			return false
		}
		str, ok := docValue.(string)
		if !ok {
			return false
		}
		prefix, ok := value.(string)
		if !ok {
			return false
		}
		return strings.HasPrefix(str, prefix)
	case vectorstore.OpEndsWith:
		if !exists {
			return false
		}
		str, ok := docValue.(string)
		if !ok {
			return false
		}
		suffix, ok := value.(string)
		if !ok {
			return false
		}
		return strings.HasSuffix(str, suffix)
	case vectorstore.OpGreaterThan, vectorstore.OpGreaterThanOrEqual, vectorstore.OpLessThan, vectorstore.OpLessThanOrEqual:
		if !exists {
			return false
		}
		return compareValues(docValue, value, op)
	}

	return false
}

// matchesTagFilter checks if document has a specific tag.
func (c *MemoryCollection) matchesTagFilter(doc *vectorstore.Document, tag string) bool {
	for _, t := range doc.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// matchesScopeFilter checks if document matches scope filter.
func (c *MemoryCollection) matchesScopeFilter(doc *vectorstore.Document, filterScope *vectorstore.Scope) bool {
	if doc.Scope == nil {
		return filterScope == nil
	}
	return doc.Scope.Match(filterScope)
}

// matchesTimeFilter checks if document matches time filter.
func (c *MemoryCollection) matchesTimeFilter(doc *vectorstore.Document, field vectorstore.TimeField, op vectorstore.FilterOperator, value time.Time) bool {
	if doc.Temporal == nil {
		return false
	}

	var docTime time.Time

	switch field {
	case vectorstore.TimeFieldCreatedAt:
		docTime = doc.Temporal.CreatedAt
	case vectorstore.TimeFieldUpdatedAt:
		docTime = doc.Temporal.UpdatedAt
	case vectorstore.TimeFieldExpiresAt:
		if doc.Temporal.ExpiresAt == nil {
			return false
		}
		docTime = *doc.Temporal.ExpiresAt
	case vectorstore.TimeFieldEventTime:
		if doc.Temporal.EventTime == nil {
			return false
		}
		docTime = *doc.Temporal.EventTime
	case vectorstore.TimeFieldValidFrom:
		if doc.Temporal.ValidFrom == nil {
			return false
		}
		docTime = *doc.Temporal.ValidFrom
	case vectorstore.TimeFieldValidUntil:
		if doc.Temporal.ValidUntil == nil {
			return false
		}
		docTime = *doc.Temporal.ValidUntil
	default:
		return false
	}

	switch op {
	case vectorstore.OpEqual:
		return docTime.Equal(value)
	case vectorstore.OpNotEqual:
		return !docTime.Equal(value)
	case vectorstore.OpGreaterThan:
		return docTime.After(value)
	case vectorstore.OpGreaterThanOrEqual:
		return docTime.Equal(value) || docTime.After(value)
	case vectorstore.OpLessThan:
		return docTime.Before(value)
	case vectorstore.OpLessThanOrEqual:
		return docTime.Equal(value) || docTime.Before(value)
	}

	return false
}

// calculateScores calculates similarity scores for candidates.
func (c *MemoryCollection) calculateScores(candidates []string, query *vectorstore.Query) []*vectorstore.Match {
	metric := query.Metric
	if metric == "" {
		metric = vectorstore.DistanceMetricCosine
	}

	matches := make([]*vectorstore.Match, 0, len(candidates))

	for _, docID := range candidates {
		doc, exists := c.documents[docID]
		if !exists || doc.Embedding == nil {
			continue
		}

		score, distance := calculateSimilarity(query.Embedding.Vector, doc.Embedding.Vector, metric)

		// Apply minimum score filter
		if query.MinScore > 0 && score < query.MinScore {
			continue
		}

		match := &vectorstore.Match{
			Document: doc,
			Score:    score,
			Distance: distance,
		}

		matches = append(matches, match)
	}

	// Sort by score (descending)
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})

	return matches
}

// cleanupExpired removes expired documents.
func (c *MemoryCollection) cleanupExpired() {
	if c.config.TTL == 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	expiredIDs := make([]string, 0)

	for id, doc := range c.documents {
		if doc.Temporal != nil && doc.Temporal.IsExpired() {
			expiredIDs = append(expiredIDs, id)
		}
	}

	for _, id := range expiredIDs {
		doc := c.documents[id]

		// Remove from indexes
		c.scopeIndex.remove(id)
		c.timeIndex.remove(id)
		c.tagIndex.remove(id)
		if c.config.EnableDeduplication {
			contentHash := calculateContentHash(doc)
			delete(c.hashIndex, contentHash)
		}

		delete(c.documents, id)
	}

	if len(expiredIDs) > 0 {
		c.updatedAt = now
	}
}

// Indexes

// scopeIndex provides indexed lookups for scope fields.
type scopeIndex struct {
	tenant  map[string][]string
	user    map[string][]string
	session map[string][]string
	agent   map[string][]string
	thread  map[string][]string
}

func newScopeIndex() *scopeIndex {
	return &scopeIndex{
		tenant:  make(map[string][]string),
		user:    make(map[string][]string),
		session: make(map[string][]string),
		agent:   make(map[string][]string),
		thread:  make(map[string][]string),
	}
}

func (idx *scopeIndex) add(docID string, scope *vectorstore.Scope) {
	if scope == nil {
		return
	}

	if scope.Tenant != "" {
		idx.tenant[scope.Tenant] = append(idx.tenant[scope.Tenant], docID)
	}
	if scope.User != "" {
		idx.user[scope.User] = append(idx.user[scope.User], docID)
	}
	if scope.Session != "" {
		idx.session[scope.Session] = append(idx.session[scope.Session], docID)
	}
	if scope.Agent != "" {
		idx.agent[scope.Agent] = append(idx.agent[scope.Agent], docID)
	}
	if scope.Thread != "" {
		idx.thread[scope.Thread] = append(idx.thread[scope.Thread], docID)
	}
}

func (idx *scopeIndex) remove(docID string) {
	// Remove from all indexes (brute force for simplicity)
	for key, ids := range idx.tenant {
		idx.tenant[key] = removeFromSlice(ids, docID)
	}
	for key, ids := range idx.user {
		idx.user[key] = removeFromSlice(ids, docID)
	}
	for key, ids := range idx.session {
		idx.session[key] = removeFromSlice(ids, docID)
	}
	for key, ids := range idx.agent {
		idx.agent[key] = removeFromSlice(ids, docID)
	}
	for key, ids := range idx.thread {
		idx.thread[key] = removeFromSlice(ids, docID)
	}
}

// timeIndex provides indexed lookups for temporal fields.
type timeIndex struct {
	createdAt map[int64][]string
	updatedAt map[int64][]string
	expiresAt map[int64][]string
}

func newTimeIndex() *timeIndex {
	return &timeIndex{
		createdAt: make(map[int64][]string),
		updatedAt: make(map[int64][]string),
		expiresAt: make(map[int64][]string),
	}
}

func (idx *timeIndex) add(docID string, temporal *vectorstore.Temporal) {
	if temporal == nil {
		return
	}

	// Use Unix timestamp for indexing (minute granularity)
	if !temporal.CreatedAt.IsZero() {
		bucket := temporal.CreatedAt.Unix() / 60
		idx.createdAt[bucket] = append(idx.createdAt[bucket], docID)
	}
	if !temporal.UpdatedAt.IsZero() {
		bucket := temporal.UpdatedAt.Unix() / 60
		idx.updatedAt[bucket] = append(idx.updatedAt[bucket], docID)
	}
	if temporal.ExpiresAt != nil {
		bucket := temporal.ExpiresAt.Unix() / 60
		idx.expiresAt[bucket] = append(idx.expiresAt[bucket], docID)
	}
}

func (idx *timeIndex) remove(docID string) {
	// Remove from all indexes (brute force for simplicity)
	for key, ids := range idx.createdAt {
		idx.createdAt[key] = removeFromSlice(ids, docID)
	}
	for key, ids := range idx.updatedAt {
		idx.updatedAt[key] = removeFromSlice(ids, docID)
	}
	for key, ids := range idx.expiresAt {
		idx.expiresAt[key] = removeFromSlice(ids, docID)
	}
}

// tagIndex provides indexed lookups for tags.
type tagIndex struct {
	tags map[string][]string
}

func newTagIndex() *tagIndex {
	return &tagIndex{
		tags: make(map[string][]string),
	}
}

func (idx *tagIndex) add(docID string, tags []string) {
	for _, tag := range tags {
		idx.tags[tag] = append(idx.tags[tag], docID)
	}
}

func (idx *tagIndex) remove(docID string) {
	for key, ids := range idx.tags {
		idx.tags[key] = removeFromSlice(ids, docID)
	}
}

// Utility functions

// calculateContentHash calculates a hash of document content.
func calculateContentHash(doc *vectorstore.Document) string {
	h := sha256.New()

	// Hash content
	if doc.Content != nil {
		h.Write([]byte(doc.Content.String()))
	}

	// Hash embedding
	if doc.Embedding != nil {
		for _, v := range doc.Embedding.Vector {
			_, _ = fmt.Fprintf(h, "%f", v)
		}
	}

	return hex.EncodeToString(h.Sum(nil))
}

// calculateSimilarity calculates similarity score and distance.
func calculateSimilarity(vec1, vec2 []float32, metric vectorstore.DistanceMetric) (score float32, distance float32) {
	switch metric {
	case vectorstore.DistanceMetricCosine:
		score = cosineSimilarity(vec1, vec2)
		distance = 1.0 - score
		return score, distance

	case vectorstore.DistanceMetricDotProduct:
		score = dotProduct(vec1, vec2)
		distance = -score // Dot product: higher is better
		return score, distance

	case vectorstore.DistanceMetricEuclidean:
		distance = euclideanDistance(vec1, vec2)
		// Convert distance to similarity (0-1 range, higher is better)
		score = 1.0 / (1.0 + distance)
		return score, distance

	default:
		// Default to cosine
		score = cosineSimilarity(vec1, vec2)
		distance = 1.0 - score
		return score, distance
	}
}

// cosineSimilarity calculates cosine similarity between two vectors.
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProd, normA, normB float32
	for i := range a {
		dotProd += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProd / (sqrt32(normA) * sqrt32(normB))
}

// dotProduct calculates dot product between two vectors.
func dotProduct(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var result float32
	for i := range a {
		result += a[i] * b[i]
	}
	return result
}

// euclideanDistance calculates Euclidean distance between two vectors.
func euclideanDistance(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var sum float32
	for i := range a {
		diff := a[i] - b[i]
		sum += diff * diff
	}
	return sqrt32(sum)
}

// sqrt32 computes square root using math.Sqrt.
func sqrt32(x float32) float32 {
	return float32(math.Sqrt(float64(x)))
}

// deepCopyDocument creates a deep copy of a document.
func deepCopyDocument(doc *vectorstore.Document) *vectorstore.Document {
	if doc == nil {
		return nil
	}

	copy := &vectorstore.Document{
		ID: doc.ID,
	}

	// Copy content
	if doc.Content != nil {
		copy.Content = &vectorstore.Content{
			Type:     doc.Content.Type,
			Text:     doc.Content.Text,
			MimeType: doc.Content.MimeType,
			URL:      doc.Content.URL,
		}
		if len(doc.Content.Data) > 0 {
			copy.Content.Data = make([]byte, len(doc.Content.Data))
			copiedBytes := copyCopy(copy.Content.Data, doc.Content.Data)
			_ = copiedBytes
		}
		if len(doc.Content.Chunks) > 0 {
			copy.Content.Chunks = make([]string, len(doc.Content.Chunks))
			copiedChunks := copyCopy(copy.Content.Chunks, doc.Content.Chunks)
			_ = copiedChunks
		}
	}

	// Copy embedding
	if doc.Embedding != nil {
		copy.Embedding = &vectorstore.Embedding{
			Model:      doc.Embedding.Model,
			Dimensions: doc.Embedding.Dimensions,
			Normalized: doc.Embedding.Normalized,
		}
		if len(doc.Embedding.Vector) > 0 {
			copy.Embedding.Vector = make([]float32, len(doc.Embedding.Vector))
			copiedVec := copyCopy(copy.Embedding.Vector, doc.Embedding.Vector)
			_ = copiedVec
		}
	}

	// Copy scope
	if doc.Scope != nil {
		copy.Scope = &vectorstore.Scope{
			Tenant:  doc.Scope.Tenant,
			User:    doc.Scope.User,
			Session: doc.Scope.Session,
			Agent:   doc.Scope.Agent,
			Thread:  doc.Scope.Thread,
		}
		if len(doc.Scope.Custom) > 0 {
			copy.Scope.Custom = make(map[string]string, len(doc.Scope.Custom))
			for k, v := range doc.Scope.Custom {
				copy.Scope.Custom[k] = v
			}
		}
	}

	// Copy temporal
	if doc.Temporal != nil {
		copy.Temporal = &vectorstore.Temporal{
			CreatedAt: doc.Temporal.CreatedAt,
			UpdatedAt: doc.Temporal.UpdatedAt,
		}
		if doc.Temporal.ExpiresAt != nil {
			expiresAt := *doc.Temporal.ExpiresAt
			copy.Temporal.ExpiresAt = &expiresAt
		}
		if doc.Temporal.EventTime != nil {
			eventTime := *doc.Temporal.EventTime
			copy.Temporal.EventTime = &eventTime
		}
		if doc.Temporal.ValidFrom != nil {
			validFrom := *doc.Temporal.ValidFrom
			copy.Temporal.ValidFrom = &validFrom
		}
		if doc.Temporal.ValidUntil != nil {
			validUntil := *doc.Temporal.ValidUntil
			copy.Temporal.ValidUntil = &validUntil
		}
	}

	// Copy tags
	if len(doc.Tags) > 0 {
		copy.Tags = make([]string, len(doc.Tags))
		copiedTags := copyCopy(copy.Tags, doc.Tags)
		_ = copiedTags
	}

	// Copy metadata
	if len(doc.Metadata) > 0 {
		copy.Metadata = make(map[string]any, len(doc.Metadata))
		for k, v := range doc.Metadata {
			copy.Metadata[k] = v
		}
	}

	return copy
}

// copyCopy is a generic copy helper.
func copyCopy[T any](dst, src []T) int {
	return copy(dst, src)
}

// compareValues compares two values based on operator.
func compareValues(a, b any, op vectorstore.FilterOperator) bool {
	// Try numeric comparison
	if af, ok := toFloat64(a); ok {
		if bf, ok := toFloat64(b); ok {
			switch op {
			case vectorstore.OpGreaterThan:
				return af > bf
			case vectorstore.OpGreaterThanOrEqual:
				return af >= bf
			case vectorstore.OpLessThan:
				return af < bf
			case vectorstore.OpLessThanOrEqual:
				return af <= bf
			}
		}
	}

	// Try string comparison
	if as, ok := a.(string); ok {
		if bs, ok := b.(string); ok {
			switch op {
			case vectorstore.OpGreaterThan:
				return as > bs
			case vectorstore.OpGreaterThanOrEqual:
				return as >= bs
			case vectorstore.OpLessThan:
				return as < bs
			case vectorstore.OpLessThanOrEqual:
				return as <= bs
			}
		}
	}

	return false
}

// toFloat64 converts various numeric types to float64.
func toFloat64(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int32:
		return float64(val), true
	case int64:
		return float64(val), true
	case uint:
		return float64(val), true
	case uint32:
		return float64(val), true
	case uint64:
		return float64(val), true
	default:
		return 0, false
	}
}

// removeFromSlice removes a string from a slice.
func removeFromSlice(slice []string, item string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}

// estimateDocumentSize estimates the storage size of a document in bytes.
func estimateDocumentSize(doc *vectorstore.Document) int64 {
	var size int64

	// ID
	size += int64(len(doc.ID))

	// Content
	if doc.Content != nil {
		size += int64(len(doc.Content.Text))
		size += int64(len(doc.Content.Data))
		size += int64(len(doc.Content.MimeType))
		size += int64(len(doc.Content.URL))
		for _, chunk := range doc.Content.Chunks {
			size += int64(len(chunk))
		}
	}

	// Embedding
	if doc.Embedding != nil {
		size += int64(len(doc.Embedding.Vector) * 4) // 4 bytes per float32
		size += int64(len(doc.Embedding.Model))
	}

	// Tags
	for _, tag := range doc.Tags {
		size += int64(len(tag))
	}

	// Metadata (rough estimate)
	size += int64(len(doc.Metadata) * 64)

	return size
}
