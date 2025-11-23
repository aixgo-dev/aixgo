package firestore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/firestore/apiv1/firestorepb"
	"github.com/aixgo-dev/aixgo/pkg/vectorstore"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// FirestoreVectorStore implements an in-production vector store using Google Cloud Firestore.
// It provides persistent, scalable vector storage with native TTL support.
//
// Features:
//   - Collection-based isolation
//   - Native Firestore TTL for automatic expiration
//   - BulkWriter for efficient batch operations
//   - Composite indexes for Scope/Temporal fields
//   - Content hash deduplication
//   - Thread-safe operations
//   - Streaming query support
//
// Important Notes:
//   - Firestore has a 500 operations per batch limit
//   - Composite indexes must be created for filtered queries
//   - Use FieldPath for safe metadata access
//   - Collections map to Firestore collections (not subcollections)
type FirestoreVectorStore struct {
	client      *firestore.Client
	projectID   string
	collections map[string]*FirestoreCollection
	mu          sync.RWMutex
}

// New creates a new FirestoreVectorStore.
//
// Options:
//   - WithProjectID(id): Set GCP project ID (required)
//   - WithCredentialsFile(path): Use service account credentials
//   - Otherwise uses Application Default Credentials
//
// Example:
//
//	store, err := firestore.New(ctx,
//	    WithProjectID("my-project"),
//	    WithCredentialsFile("/path/to/credentials.json"),
//	)
func New(ctx context.Context, opts ...Option) (vectorstore.VectorStore, error) {
	config := &Config{}
	for _, opt := range opts {
		opt(config)
	}

	if config.ProjectID == "" {
		return nil, fmt.Errorf("project ID is required")
	}

	var clientOpts []option.ClientOption
	if config.CredentialsFile != "" {
		clientOpts = append(clientOpts, option.WithCredentialsFile(config.CredentialsFile))
	}

	client, err := firestore.NewClient(ctx, config.ProjectID, clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create firestore client: %w", err)
	}

	return &FirestoreVectorStore{
		client:      client,
		projectID:   config.ProjectID,
		collections: make(map[string]*FirestoreCollection),
	}, nil
}

// Config contains configuration for the Firestore vector store.
type Config struct {
	ProjectID       string
	CredentialsFile string
}

// Option configures a FirestoreVectorStore.
type Option func(*Config)

// WithProjectID sets the GCP project ID.
func WithProjectID(projectID string) Option {
	return func(c *Config) {
		c.ProjectID = projectID
	}
}

// WithCredentialsFile sets the path to service account credentials.
func WithCredentialsFile(path string) Option {
	return func(c *Config) {
		c.CredentialsFile = path
	}
}

// Collection returns a collection with the specified name and options.
func (f *FirestoreVectorStore) Collection(name string, opts ...vectorstore.CollectionOption) vectorstore.Collection {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Return existing collection if already created
	if coll, exists := f.collections[name]; exists {
		return coll
	}

	// Create new collection
	config := vectorstore.ApplyOptions(opts)
	coll := &FirestoreCollection{
		name:      name,
		config:    config,
		client:    f.client,
		collRef:   f.client.Collection(name),
		createdAt: time.Now(),
		updatedAt: time.Now(),
	}

	f.collections[name] = coll
	return coll
}

// ListCollections returns the names of all collections.
func (f *FirestoreVectorStore) ListCollections(ctx context.Context) ([]string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	names := make([]string, 0, len(f.collections))
	for name := range f.collections {
		names = append(names, name)
	}

	sort.Strings(names)
	return names, nil
}

// DeleteCollection permanently deletes a collection and all its documents.
func (f *FirestoreVectorStore) DeleteCollection(ctx context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.collections[name]; !exists {
		return fmt.Errorf("collection %q does not exist", name)
	}

	// Delete all documents in the collection
	// Firestore requires deleting documents before collection
	collRef := f.client.Collection(name)
	bulkWriter := f.client.BulkWriter(ctx)

	iter := collRef.Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			bulkWriter.End()
			return fmt.Errorf("failed to iterate documents: %w", err)
		}

		if _, err := bulkWriter.Delete(doc.Ref); err != nil {
			bulkWriter.End()
			return fmt.Errorf("failed to queue delete: %w", err)
		}
	}

	bulkWriter.End()

	delete(f.collections, name)
	return nil
}

// Stats returns statistics about the vector store.
func (f *FirestoreVectorStore) Stats(ctx context.Context) (*vectorstore.StoreStats, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var totalDocs int64
	for _, coll := range f.collections {
		stats, err := coll.Stats(ctx)
		if err != nil {
			return nil, err
		}
		totalDocs += stats.Documents
	}

	return &vectorstore.StoreStats{
		Collections:  int64(len(f.collections)),
		Documents:    totalDocs,
		StorageBytes: 0, // Firestore doesn't expose storage size
		Provider:     "firestore",
		Version:      "1.0.0",
		Extra: map[string]any{
			"project_id": f.projectID,
		},
	}, nil
}

// Close closes the connection to Firestore.
func (f *FirestoreVectorStore) Close() error {
	return f.client.Close()
}

// FirestoreCollection represents a collection in Firestore.
type FirestoreCollection struct {
	name      string
	config    *vectorstore.CollectionConfig
	client    *firestore.Client
	collRef   *firestore.CollectionRef
	createdAt time.Time
	updatedAt time.Time
	mu        sync.RWMutex
}

// firestoreDocument represents the structure of a document in Firestore.
// All fields are stored at the top level for efficient querying.
type firestoreDocument struct {
	// Core fields
	ID string `firestore:"id"`

	// Content fields
	ContentType     string   `firestore:"content_type"`
	ContentText     string   `firestore:"content_text,omitempty"`
	ContentData     []byte   `firestore:"content_data,omitempty"`
	ContentMimeType string   `firestore:"content_mimetype,omitempty"`
	ContentURL      string   `firestore:"content_url,omitempty"`
	ContentChunks   []string `firestore:"content_chunks,omitempty"`

	// Embedding fields
	Embedding          interface{} `firestore:"embedding"` // Firestore vector type
	EmbeddingModel     string      `firestore:"embedding_model,omitempty"`
	EmbeddingDimension int         `firestore:"embedding_dimension,omitempty"`
	EmbeddingNormalize bool        `firestore:"embedding_normalized,omitempty"`

	// Scope fields (flattened for indexing)
	ScopeTenant  string            `firestore:"scope_tenant,omitempty"`
	ScopeUser    string            `firestore:"scope_user,omitempty"`
	ScopeSession string            `firestore:"scope_session,omitempty"`
	ScopeAgent   string            `firestore:"scope_agent,omitempty"`
	ScopeThread  string            `firestore:"scope_thread,omitempty"`
	ScopeCustom  map[string]string `firestore:"scope_custom,omitempty"`

	// Temporal fields
	CreatedAt   time.Time  `firestore:"created_at"`
	UpdatedAt   time.Time  `firestore:"updated_at"`
	ExpiresAt   *time.Time `firestore:"expires_at,omitempty"`
	EventTime   *time.Time `firestore:"event_time,omitempty"`
	ValidFrom   *time.Time `firestore:"valid_from,omitempty"`
	ValidUntil  *time.Time `firestore:"valid_until,omitempty"`
	TTLFieldSet bool       `firestore:"ttl_field_set,omitempty"` // For native TTL

	// Tags and metadata
	Tags     []string               `firestore:"tags,omitempty"`
	Metadata map[string]interface{} `firestore:"metadata,omitempty"`

	// Deduplication
	ContentHash string `firestore:"content_hash,omitempty"`
}

// Name returns the collection name.
func (c *FirestoreCollection) Name() string {
	return c.name
}

// Upsert inserts or updates documents in the collection.
func (c *FirestoreCollection) Upsert(ctx context.Context, documents ...*vectorstore.Document) (*vectorstore.UpsertResult, error) {
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

	// Use BulkWriter for efficient batch operations
	// Note: Firestore has a 500 operations per batch limit
	bulkWriter := c.client.BulkWriter(ctx)
	defer bulkWriter.End()

	for _, doc := range documents {
		// Check deduplication
		if c.config.EnableDeduplication {
			contentHash := calculateContentHash(doc)

			// Query for existing document with same hash
			query := c.collRef.Where("content_hash", "==", contentHash).Limit(1)
			iter := query.Documents(ctx)
			existingDoc, err := iter.Next()
			iter.Stop()

			if err == nil && existingDoc != nil && existingDoc.Ref.ID != doc.ID {
				// Found a duplicate with different ID
				var fsDoc firestoreDocument
				if err := existingDoc.DataTo(&fsDoc); err == nil {
					// Convert to vectorstore document to check similarity
					existingVSDoc := c.firestoreToVectorstoreDoc(&fsDoc)
					if doc.Embedding != nil && existingVSDoc.Embedding != nil {
						similarity := cosineSimilarity(doc.Embedding.Vector, existingVSDoc.Embedding.Vector)
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
		docRef := c.collRef.Doc(doc.ID)
		docSnap, err := docRef.Get(ctx)
		exists := err == nil && docSnap.Exists()

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

		// Convert to Firestore document
		fsDoc := c.vectorstoreToFirestoreDoc(doc)

		// Queue write
		if _, err := bulkWriter.Set(docRef, fsDoc); err != nil {
			return nil, fmt.Errorf("failed to queue document %s: %w", doc.ID, err)
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
func (c *FirestoreCollection) UpsertBatch(ctx context.Context, documents []*vectorstore.Document, opts ...vectorstore.BatchOption) (*vectorstore.UpsertResult, error) {
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

	// Firestore has a 500 operations per batch limit
	maxBatchSize := 500
	if config.BatchSize > maxBatchSize {
		config.BatchSize = maxBatchSize
	}

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
func (c *FirestoreCollection) Query(ctx context.Context, query *vectorstore.Query) (*vectorstore.QueryResult, error) {
	if err := query.Validate(); err != nil {
		return nil, fmt.Errorf("invalid query: %w", err)
	}

	startTime := time.Now()
	timing := &vectorstore.QueryTiming{}

	c.mu.RLock()
	defer c.mu.RUnlock()

	// Start with base query
	fsQuery := c.collRef.Query

	// Apply filters
	filterStart := time.Now()
	fsQuery = c.applyFilters(fsQuery, query.Filters)
	timing.FilterApplication = time.Since(filterStart)

	// Execute query
	retrievalStart := time.Now()
	iter := fsQuery.Documents(ctx)

	var fsDocs []*firestoreDocument
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to iterate documents: %w", err)
		}

		var fsDoc firestoreDocument
		if err := doc.DataTo(&fsDoc); err != nil {
			return nil, fmt.Errorf("failed to unmarshal document: %w", err)
		}

		fsDocs = append(fsDocs, &fsDoc)
	}
	timing.Retrieval = time.Since(retrievalStart)

	// Convert to vectorstore documents and calculate scores
	scoringStart := time.Now()
	var matches []*vectorstore.Match

	for _, fsDoc := range fsDocs {
		doc := c.firestoreToVectorstoreDoc(fsDoc)

		if query.Embedding != nil && doc.Embedding != nil {
			score, distance := calculateSimilarity(query.Embedding.Vector, doc.Embedding.Vector, query.Metric)

			// Apply minimum score filter
			if query.MinScore > 0 && score < query.MinScore {
				continue
			}

			matches = append(matches, &vectorstore.Match{
				Document: doc,
				Score:    score,
				Distance: distance,
			})
		} else {
			// Filter-only query
			matches = append(matches, &vectorstore.Match{
				Document: doc,
				Score:    1.0,
			})
		}
	}

	// Sort by score (descending)
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})

	timing.Scoring = time.Since(scoringStart)

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
func (c *FirestoreCollection) QueryStream(ctx context.Context, query *vectorstore.Query) (vectorstore.ResultIterator, error) {
	// For Firestore, we materialize all results and return a slice iterator
	// A true streaming implementation would use Firestore snapshots
	result, err := c.Query(ctx, query)
	if err != nil {
		return vectorstore.NewErrorIterator(err), nil
	}

	return vectorstore.NewSliceIterator(result.Matches), nil
}

// Get retrieves documents by their IDs.
func (c *FirestoreCollection) Get(ctx context.Context, ids ...string) ([]*vectorstore.Document, error) {
	if len(ids) == 0 {
		return []*vectorstore.Document{}, nil
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	documents := make([]*vectorstore.Document, 0, len(ids))

	for _, id := range ids {
		docRef := c.collRef.Doc(id)
		docSnap, err := docRef.Get(ctx)

		if err != nil {
			if status.Code(err) == codes.NotFound {
				// Document not found, skip it
				continue
			}
			return nil, fmt.Errorf("failed to get document %s: %w", id, err)
		}

		var fsDoc firestoreDocument
		if err := docSnap.DataTo(&fsDoc); err != nil {
			return nil, fmt.Errorf("failed to unmarshal document %s: %w", id, err)
		}

		doc := c.firestoreToVectorstoreDoc(&fsDoc)
		documents = append(documents, doc)
	}

	return documents, nil
}

// Delete removes documents by their IDs.
func (c *FirestoreCollection) Delete(ctx context.Context, ids ...string) (*vectorstore.DeleteResult, error) {
	if len(ids) == 0 {
		return &vectorstore.DeleteResult{}, nil
	}

	startTime := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	result := &vectorstore.DeleteResult{}

	// Use BulkWriter for efficient batch deletes
	bulkWriter := c.client.BulkWriter(ctx)
	defer bulkWriter.End()

	for _, id := range ids {
		docRef := c.collRef.Doc(id)

		// Check if document exists
		docSnap, err := docRef.Get(ctx)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				result.NotFound++
				result.NotFoundIDs = append(result.NotFoundIDs, id)
				continue
			}
			return nil, fmt.Errorf("failed to check document %s: %w", id, err)
		}

		if !docSnap.Exists() {
			result.NotFound++
			result.NotFoundIDs = append(result.NotFoundIDs, id)
			continue
		}

		// Queue delete
		if _, err := bulkWriter.Delete(docRef); err != nil {
			return nil, fmt.Errorf("failed to queue delete for document %s: %w", id, err)
		}

		result.Deleted++
	}

	c.updatedAt = time.Now()

	result.Timing = &vectorstore.OperationTiming{
		Total: time.Since(startTime),
	}

	return result, nil
}

// DeleteByFilter removes all documents matching the filter.
func (c *FirestoreCollection) DeleteByFilter(ctx context.Context, filter vectorstore.Filter) (*vectorstore.DeleteResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	startTime := time.Now()

	// Build query with filters
	fsQuery := c.collRef.Query
	fsQuery = c.applyFilters(fsQuery, filter)

	// Get all matching documents
	iter := fsQuery.Documents(ctx)

	result := &vectorstore.DeleteResult{}
	bulkWriter := c.client.BulkWriter(ctx)
	defer bulkWriter.End()

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to iterate documents: %w", err)
		}

		// Queue delete
		if _, err := bulkWriter.Delete(doc.Ref); err != nil {
			return nil, fmt.Errorf("failed to queue delete: %w", err)
		}

		result.Deleted++
	}

	c.updatedAt = time.Now()

	result.Timing = &vectorstore.OperationTiming{
		Total: time.Since(startTime),
	}

	return result, nil
}

// Count returns the number of documents in the collection.
func (c *FirestoreCollection) Count(ctx context.Context, filter vectorstore.Filter) (int64, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	fsQuery := c.collRef.Query
	if filter != nil {
		fsQuery = c.applyFilters(fsQuery, filter)
	}

	// Use AggregationQuery for efficient counting
	aggQuery := fsQuery.NewAggregationQuery().WithCount("count")
	results, err := aggQuery.Get(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to count documents: %w", err)
	}

	count, ok := results["count"]
	if !ok {
		return 0, nil
	}

	countValue, ok := count.(*firestorepb.Value)
	if !ok {
		return 0, nil
	}

	return countValue.GetIntegerValue(), nil
}

// Stats returns statistics about the collection.
func (c *FirestoreCollection) Stats(ctx context.Context) (*vectorstore.CollectionStats, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Count documents
	count, err := c.Count(ctx, nil)
	if err != nil {
		return nil, err
	}

	// Get sample document to determine embedding dimensions
	var embeddingDims int
	iter := c.collRef.Limit(1).Documents(ctx)
	doc, err := iter.Next()
	if err == nil {
		var fsDoc firestoreDocument
		if err := doc.DataTo(&fsDoc); err == nil {
			embeddingDims = fsDoc.EmbeddingDimension
		}
	}
	iter.Stop()

	return &vectorstore.CollectionStats{
		Name:                c.name,
		Documents:           count,
		StorageBytes:        0, // Firestore doesn't expose storage size
		EmbeddingDimensions: embeddingDims,
		IndexType:           string(c.config.IndexType),
		CreatedAt:           vectorstore.NewTimestamp(c.createdAt),
		UpdatedAt:           vectorstore.NewTimestamp(c.updatedAt),
	}, nil
}

// Clear removes all documents from the collection.
func (c *FirestoreCollection) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	bulkWriter := c.client.BulkWriter(ctx)
	defer bulkWriter.End()

	iter := c.collRef.Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to iterate documents: %w", err)
		}

		if _, err := bulkWriter.Delete(doc.Ref); err != nil {
			return fmt.Errorf("failed to queue delete: %w", err)
		}
	}

	c.updatedAt = time.Now()
	return nil
}

// Helper methods

// validateRequiredScope validates that document has required scope fields.
func (c *FirestoreCollection) validateRequiredScope(doc *vectorstore.Document) error {
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

// applyFilters applies filters to a Firestore query.
// Note: Firestore has limitations on OR queries - they require multiple queries and merging.
// Composite indexes must be created for filtered queries in production.
func (c *FirestoreCollection) applyFilters(query firestore.Query, filter vectorstore.Filter) firestore.Query {
	if filter == nil {
		return query
	}

	// Handle composite filters
	if vectorstore.IsAndFilter(filter) {
		filters := vectorstore.GetFilters(filter)
		for _, f := range filters {
			query = c.applyFilters(query, f)
		}
		return query
	}

	// OR filters require multiple queries - not directly supported here
	// In production, you'd run multiple queries and merge results
	if vectorstore.IsOrFilter(filter) {
		// For simplicity, we'll apply the first filter only
		// A full implementation would run multiple queries and merge results
		filters := vectorstore.GetFilters(filter)
		if len(filters) > 0 {
			query = c.applyFilters(query, filters[0])
		}
		return query
	}

	if vectorstore.IsNotFilter(filter) {
		// NOT is challenging in Firestore - requires client-side filtering
		// For now, we skip it
		return query
	}

	// Handle field filters
	if field, op, value, ok := vectorstore.GetFieldFilter(filter); ok {
		return c.applyFieldFilter(query, field, op, value)
	}

	// Handle tag filters
	if tag, ok := vectorstore.GetTagFilter(filter); ok {
		return query.Where("tags", "array-contains", tag)
	}

	// Handle scope filters
	if scope, ok := vectorstore.GetScopeFilter(filter); ok {
		if scope.Tenant != "" {
			query = query.Where("scope_tenant", "==", scope.Tenant)
		}
		if scope.User != "" {
			query = query.Where("scope_user", "==", scope.User)
		}
		if scope.Session != "" {
			query = query.Where("scope_session", "==", scope.Session)
		}
		if scope.Agent != "" {
			query = query.Where("scope_agent", "==", scope.Agent)
		}
		if scope.Thread != "" {
			query = query.Where("scope_thread", "==", scope.Thread)
		}
		return query
	}

	// Handle time filters
	if field, op, value, ok := vectorstore.GetTimeFilter(filter); ok {
		return c.applyTimeFilter(query, field, op, value)
	}

	// Score filters are applied during scoring, not filtering
	return query
}

// applyFieldFilter applies a field filter to a Firestore query.
func (c *FirestoreCollection) applyFieldFilter(query firestore.Query, field string, op vectorstore.FilterOperator, value any) firestore.Query {
	// Use FieldPath for safe metadata access
	fieldPath := fmt.Sprintf("metadata.%s", field)

	switch op {
	case vectorstore.OpEqual:
		return query.Where(fieldPath, "==", value)
	case vectorstore.OpNotEqual:
		return query.Where(fieldPath, "!=", value)
	case vectorstore.OpGreaterThan:
		return query.Where(fieldPath, ">", value)
	case vectorstore.OpGreaterThanOrEqual:
		return query.Where(fieldPath, ">=", value)
	case vectorstore.OpLessThan:
		return query.Where(fieldPath, "<", value)
	case vectorstore.OpLessThanOrEqual:
		return query.Where(fieldPath, "<=", value)
	case vectorstore.OpIn:
		return query.Where(fieldPath, "in", value)
	case vectorstore.OpNotIn:
		return query.Where(fieldPath, "not-in", value)
	// String operations require client-side filtering in Firestore
	case vectorstore.OpContains, vectorstore.OpStartsWith, vectorstore.OpEndsWith:
		// Not directly supported - requires client-side filtering
		return query
	}

	return query
}

// applyTimeFilter applies a time filter to a Firestore query.
func (c *FirestoreCollection) applyTimeFilter(query firestore.Query, field vectorstore.TimeField, op vectorstore.FilterOperator, value time.Time) firestore.Query {
	var firestoreField string

	switch field {
	case vectorstore.TimeFieldCreatedAt:
		firestoreField = "created_at"
	case vectorstore.TimeFieldUpdatedAt:
		firestoreField = "updated_at"
	case vectorstore.TimeFieldExpiresAt:
		firestoreField = "expires_at"
	case vectorstore.TimeFieldEventTime:
		firestoreField = "event_time"
	case vectorstore.TimeFieldValidFrom:
		firestoreField = "valid_from"
	case vectorstore.TimeFieldValidUntil:
		firestoreField = "valid_until"
	default:
		return query
	}

	switch op {
	case vectorstore.OpEqual:
		return query.Where(firestoreField, "==", value)
	case vectorstore.OpNotEqual:
		return query.Where(firestoreField, "!=", value)
	case vectorstore.OpGreaterThan:
		return query.Where(firestoreField, ">", value)
	case vectorstore.OpGreaterThanOrEqual:
		return query.Where(firestoreField, ">=", value)
	case vectorstore.OpLessThan:
		return query.Where(firestoreField, "<", value)
	case vectorstore.OpLessThanOrEqual:
		return query.Where(firestoreField, "<=", value)
	}

	return query
}

// vectorstoreToFirestoreDoc converts a vectorstore.Document to firestoreDocument.
func (c *FirestoreCollection) vectorstoreToFirestoreDoc(doc *vectorstore.Document) *firestoreDocument {
	fsDoc := &firestoreDocument{
		ID:       doc.ID,
		Tags:     doc.Tags,
		Metadata: make(map[string]interface{}),
	}

	// Convert content
	if doc.Content != nil {
		fsDoc.ContentType = string(doc.Content.Type)
		fsDoc.ContentText = doc.Content.Text
		fsDoc.ContentData = doc.Content.Data
		fsDoc.ContentMimeType = doc.Content.MimeType
		fsDoc.ContentURL = doc.Content.URL
		fsDoc.ContentChunks = doc.Content.Chunks
	}

	// Convert embedding to Firestore vector type
	if doc.Embedding != nil {
		fsDoc.Embedding = float32SliceToFirestoreArray(doc.Embedding.Vector)
		fsDoc.EmbeddingModel = doc.Embedding.Model
		fsDoc.EmbeddingDimension = doc.Embedding.Dimensions
		fsDoc.EmbeddingNormalize = doc.Embedding.Normalized
	}

	// Flatten scope for indexing
	if doc.Scope != nil {
		fsDoc.ScopeTenant = doc.Scope.Tenant
		fsDoc.ScopeUser = doc.Scope.User
		fsDoc.ScopeSession = doc.Scope.Session
		fsDoc.ScopeAgent = doc.Scope.Agent
		fsDoc.ScopeThread = doc.Scope.Thread
		fsDoc.ScopeCustom = doc.Scope.Custom
	}

	// Map temporal fields
	if doc.Temporal != nil {
		fsDoc.CreatedAt = doc.Temporal.CreatedAt
		fsDoc.UpdatedAt = doc.Temporal.UpdatedAt
		fsDoc.ExpiresAt = doc.Temporal.ExpiresAt
		fsDoc.EventTime = doc.Temporal.EventTime
		fsDoc.ValidFrom = doc.Temporal.ValidFrom
		fsDoc.ValidUntil = doc.Temporal.ValidUntil

		// Set TTL field for native Firestore TTL
		if doc.Temporal.ExpiresAt != nil {
			fsDoc.TTLFieldSet = true
		}
	}

	// Copy metadata
	for k, v := range doc.Metadata {
		fsDoc.Metadata[k] = v
	}

	// Calculate content hash for deduplication
	if c.config.EnableDeduplication {
		fsDoc.ContentHash = calculateContentHash(doc)
	}

	return fsDoc
}

// firestoreToVectorstoreDoc converts a firestoreDocument to vectorstore.Document.
func (c *FirestoreCollection) firestoreToVectorstoreDoc(fsDoc *firestoreDocument) *vectorstore.Document {
	doc := &vectorstore.Document{
		ID:       fsDoc.ID,
		Tags:     fsDoc.Tags,
		Metadata: fsDoc.Metadata,
	}

	// Convert content
	if fsDoc.ContentType != "" {
		doc.Content = &vectorstore.Content{
			Type:     vectorstore.ContentType(fsDoc.ContentType),
			Text:     fsDoc.ContentText,
			Data:     fsDoc.ContentData,
			MimeType: fsDoc.ContentMimeType,
			URL:      fsDoc.ContentURL,
			Chunks:   fsDoc.ContentChunks,
		}
	}

	// Convert embedding
	if fsDoc.Embedding != nil {
		vector := extractEmbeddingFromFirestore(fsDoc.Embedding)
		if vector != nil {
			doc.Embedding = &vectorstore.Embedding{
				Vector:     vector,
				Model:      fsDoc.EmbeddingModel,
				Dimensions: fsDoc.EmbeddingDimension,
				Normalized: fsDoc.EmbeddingNormalize,
			}
		}
	}

	// Reconstruct scope
	if fsDoc.ScopeTenant != "" || fsDoc.ScopeUser != "" || fsDoc.ScopeSession != "" ||
		fsDoc.ScopeAgent != "" || fsDoc.ScopeThread != "" || len(fsDoc.ScopeCustom) > 0 {
		doc.Scope = &vectorstore.Scope{
			Tenant:  fsDoc.ScopeTenant,
			User:    fsDoc.ScopeUser,
			Session: fsDoc.ScopeSession,
			Agent:   fsDoc.ScopeAgent,
			Thread:  fsDoc.ScopeThread,
			Custom:  fsDoc.ScopeCustom,
		}
	}

	// Reconstruct temporal
	if !fsDoc.CreatedAt.IsZero() || !fsDoc.UpdatedAt.IsZero() {
		doc.Temporal = &vectorstore.Temporal{
			CreatedAt:  fsDoc.CreatedAt,
			UpdatedAt:  fsDoc.UpdatedAt,
			ExpiresAt:  fsDoc.ExpiresAt,
			EventTime:  fsDoc.EventTime,
			ValidFrom:  fsDoc.ValidFrom,
			ValidUntil: fsDoc.ValidUntil,
		}
	}

	return doc
}

// Utility functions

// float32SliceToFirestoreArray converts a float32 slice to Firestore array format.
func float32SliceToFirestoreArray(slice []float32) []*firestorepb.Value {
	values := make([]*firestorepb.Value, len(slice))
	for i, v := range slice {
		values[i] = &firestorepb.Value{
			ValueType: &firestorepb.Value_DoubleValue{
				DoubleValue: float64(v),
			},
		}
	}
	return values
}

// extractEmbeddingFromFirestore extracts embedding vector from Firestore format.
func extractEmbeddingFromFirestore(embedding interface{}) []float32 {
	if embedding == nil {
		return nil
	}

	// Handle Firestore protobuf Value
	if pbValue, ok := embedding.(*firestorepb.Value); ok {
		if arrayVal := pbValue.GetArrayValue(); arrayVal != nil {
			result := make([]float32, len(arrayVal.Values))
			for i, val := range arrayVal.Values {
				result[i] = float32(val.GetDoubleValue())
			}
			return result
		}
	}

	// Handle direct array
	if arr, ok := embedding.([]*firestorepb.Value); ok {
		result := make([]float32, len(arr))
		for i, val := range arr {
			result[i] = float32(val.GetDoubleValue())
		}
		return result
	}

	// Handle []interface{} which might contain float64 values
	if slice, ok := embedding.([]interface{}); ok {
		result := make([]float32, len(slice))
		for i, v := range slice {
			switch val := v.(type) {
			case float64:
				result[i] = float32(val)
			case float32:
				result[i] = val
			case int:
				result[i] = float32(val)
			case int64:
				result[i] = float32(val)
			default:
				return nil
			}
		}
		return result
	}

	// Handle []float32
	if slice, ok := embedding.([]float32); ok {
		return slice
	}

	return nil
}

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
	if metric == "" {
		metric = vectorstore.DistanceMetricCosine
	}

	switch metric {
	case vectorstore.DistanceMetricCosine:
		score = cosineSimilarity(vec1, vec2)
		distance = 1.0 - score
		return score, distance

	case vectorstore.DistanceMetricDotProduct:
		score = dotProduct(vec1, vec2)
		distance = -score
		return score, distance

	case vectorstore.DistanceMetricEuclidean:
		distance = euclideanDistance(vec1, vec2)
		score = 1.0 / (1.0 + distance)
		return score, distance

	default:
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

	return dotProd / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
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
	return float32(math.Sqrt(float64(sum)))
}
