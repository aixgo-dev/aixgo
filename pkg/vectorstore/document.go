package vectorstore

import (
	"encoding/base64"
	"fmt"
	"time"
)

// Document represents a document with embeddings, content, and metadata.
// Documents are the primary unit of storage in a vector database.
//
// Unlike the previous version which only had a string Content field,
// this enhanced Document supports:
//   - Multi-modal content (text, image, audio, video)
//   - Typed scope fields for multi-tenancy
//   - Temporal information for time-based queries
//   - Tags for efficient filtering
//   - Structured metadata separate from free-form data
type Document struct {
	// ID is the unique identifier for the document.
	// Must be unique within a collection.
	// IDs should be URL-safe strings (alphanumeric, hyphens, underscores).
	ID string

	// Content is the multi-modal content of the document.
	// Supports text, images, audio, video.
	Content *Content

	// Embedding is the vector representation of the content.
	// Can be nil if embeddings are generated server-side.
	Embedding *Embedding

	// Scope defines hierarchical context for the document.
	// Useful for multi-tenancy, user isolation, session tracking.
	// Example: {Tenant: "acme", User: "user123", Session: "sess456"}
	Scope *Scope

	// Temporal contains time-related information for the document.
	// Useful for TTL, time-based queries, event ordering.
	Temporal *Temporal

	// Tags are indexed labels for efficient filtering.
	// Unlike metadata, tags are optimized for equality queries.
	// Examples: ["product", "electronics", "featured"]
	Tags []string

	// Metadata contains additional free-form information.
	// Use this for data that doesn't fit into typed fields.
	// Keys should be alphanumeric with underscores (no special chars).
	Metadata map[string]any

	// Score is the similarity score (populated during queries).
	// Not stored, only returned in query results.
	Score float32 `json:"-"`

	// Distance is the raw distance metric (populated during queries).
	// Not stored, only returned in query results.
	Distance float32 `json:"-"`
}

// Content represents multi-modal document content.
// A document can contain text, images, audio, or video.
type Content struct {
	// Type indicates the content type
	Type ContentType

	// Text is the text content (for ContentTypeText)
	Text string

	// Data is the binary data (for images, audio, video)
	// Stored as base64-encoded string for JSON serialization
	Data []byte

	// MimeType is the MIME type of the content (e.g., "image/jpeg", "audio/mp3")
	MimeType string

	// URL is an optional external URL for the content
	// Useful for referencing large media without storing it inline
	URL string

	// Chunks contains text chunks for long documents
	// Useful for document splitting and retrieval
	Chunks []string
}

// ContentType represents the type of content in a document.
type ContentType string

const (
	// ContentTypeText represents text content
	ContentTypeText ContentType = "text"

	// ContentTypeImage represents image content (JPEG, PNG, WebP, etc.)
	ContentTypeImage ContentType = "image"

	// ContentTypeAudio represents audio content (MP3, WAV, etc.)
	ContentTypeAudio ContentType = "audio"

	// ContentTypeVideo represents video content (MP4, WebM, etc.)
	ContentTypeVideo ContentType = "video"

	// ContentTypeMultimodal represents mixed content types
	ContentTypeMultimodal ContentType = "multimodal"
)

// NewTextContent creates a Content with text.
func NewTextContent(text string) *Content {
	return &Content{
		Type: ContentTypeText,
		Text: text,
	}
}

// NewImageContent creates a Content with image data.
func NewImageContent(data []byte, mimeType string) *Content {
	return &Content{
		Type:     ContentTypeImage,
		Data:     data,
		MimeType: mimeType,
	}
}

// NewImageURL creates a Content with an image URL reference.
func NewImageURL(url string) *Content {
	return &Content{
		Type:     ContentTypeImage,
		URL:      url,
		MimeType: "image/*",
	}
}

// NewAudioContent creates a Content with audio data.
func NewAudioContent(data []byte, mimeType string) *Content {
	return &Content{
		Type:     ContentTypeAudio,
		Data:     data,
		MimeType: mimeType,
	}
}

// NewVideoContent creates a Content with video data.
func NewVideoContent(data []byte, mimeType string) *Content {
	return &Content{
		Type:     ContentTypeVideo,
		Data:     data,
		MimeType: mimeType,
	}
}

// String returns a string representation of the content.
// For text, returns the text. For binary, returns a summary.
func (c *Content) String() string {
	if c == nil {
		return ""
	}

	switch c.Type {
	case ContentTypeText:
		return c.Text
	case ContentTypeImage, ContentTypeAudio, ContentTypeVideo:
		if c.URL != "" {
			return fmt.Sprintf("[%s: %s]", c.Type, c.URL)
		}
		if len(c.Data) > 0 {
			return fmt.Sprintf("[%s: %d bytes]", c.Type, len(c.Data))
		}
		return fmt.Sprintf("[%s]", c.Type)
	default:
		return fmt.Sprintf("[%s]", c.Type)
	}
}

// DataBase64 returns the binary data as a base64-encoded string.
// This is useful for JSON serialization.
func (c *Content) DataBase64() string {
	if len(c.Data) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(c.Data)
}

// Embedding represents a vector embedding with metadata.
type Embedding struct {
	// Vector is the embedding vector
	Vector []float32

	// Model is the name of the model that generated this embedding
	// Examples: "text-embedding-3-small", "clip-vit-base-patch32"
	Model string

	// Dimensions is the dimensionality of the vector
	// Automatically set from len(Vector)
	Dimensions int

	// Normalized indicates whether the vector is normalized (unit length)
	// Many distance metrics work better with normalized vectors
	Normalized bool
}

// NewEmbedding creates a new Embedding from a vector and model name.
func NewEmbedding(vector []float32, model string) *Embedding {
	return &Embedding{
		Vector:     vector,
		Model:      model,
		Dimensions: len(vector),
		Normalized: false,
	}
}

// NewNormalizedEmbedding creates a normalized embedding (unit length).
// The vector is normalized in-place.
func NewNormalizedEmbedding(vector []float32, model string) *Embedding {
	normalizeVector(vector)
	return &Embedding{
		Vector:     vector,
		Model:      model,
		Dimensions: len(vector),
		Normalized: true,
	}
}

// Normalize normalizes the embedding vector to unit length (in-place).
func (e *Embedding) Normalize() {
	if e.Normalized {
		return
	}
	normalizeVector(e.Vector)
	e.Normalized = true
}

// normalizeVector normalizes a vector to unit length (in-place).
func normalizeVector(v []float32) {
	var sumSquares float32
	for _, val := range v {
		sumSquares += val * val
	}

	if sumSquares == 0 {
		return // Zero vector, nothing to normalize
	}

	norm := float32(1.0) / sqrt32(sumSquares)
	for i := range v {
		v[i] *= norm
	}
}

// sqrt32 computes the square root of a float32.
func sqrt32(x float32) float32 {
	// Fast approximate square root using bit manipulation
	// This is faster than converting to float64 and using math.Sqrt
	if x == 0 {
		return 0
	}

	// Use Newton-Raphson method for better accuracy
	// Initial guess using bit manipulation
	xhalf := 0.5 * x
	i := int32(0x5f3759df - (int32(x) >> 1)) // Magic constant from Quake III
	x = float32(i)
	x = x * (1.5 - (xhalf * x * x)) // One iteration
	x = x * (1.5 - (xhalf * x * x)) // Second iteration for better accuracy
	return 1.0 / x
}

// Scope defines hierarchical context for a document.
// This enables multi-tenancy, user isolation, and session tracking.
//
// Scope fields are indexed separately for efficient filtering.
// All fields are optional and can be combined as needed.
type Scope struct {
	// Tenant is the top-level scope (organization, workspace, team)
	Tenant string

	// User is the user identifier
	User string

	// Session is the session identifier
	Session string

	// Agent is the agent identifier (for multi-agent systems)
	Agent string

	// Thread is the conversation thread identifier
	Thread string

	// Custom contains additional custom scope dimensions
	// Use this for domain-specific scoping needs
	Custom map[string]string
}

// NewScope creates a scope with common fields.
func NewScope(tenant, user, session string) *Scope {
	return &Scope{
		Tenant:  tenant,
		User:    user,
		Session: session,
	}
}

// Match checks if this scope matches another scope.
// A nil field matches any value (wildcard).
func (s *Scope) Match(other *Scope) bool {
	if s == nil || other == nil {
		return true // nil scope matches everything
	}

	if s.Tenant != "" && other.Tenant != "" && s.Tenant != other.Tenant {
		return false
	}
	if s.User != "" && other.User != "" && s.User != other.User {
		return false
	}
	if s.Session != "" && other.Session != "" && s.Session != other.Session {
		return false
	}
	if s.Agent != "" && other.Agent != "" && s.Agent != other.Agent {
		return false
	}
	if s.Thread != "" && other.Thread != "" && s.Thread != other.Thread {
		return false
	}

	return true
}

// Temporal contains time-related information for a document.
// This enables TTL, time-based queries, and event ordering.
type Temporal struct {
	// CreatedAt is when the document was created
	CreatedAt time.Time

	// UpdatedAt is when the document was last updated
	UpdatedAt time.Time

	// ExpiresAt is when the document should expire (optional)
	// Used for automatic cleanup in caching scenarios
	ExpiresAt *time.Time

	// EventTime is the time of the event this document represents (optional)
	// Used for time-series data and event ordering
	EventTime *time.Time

	// ValidFrom is when this document becomes valid (optional)
	// Used for future-dated content
	ValidFrom *time.Time

	// ValidUntil is when this document stops being valid (optional)
	// Different from ExpiresAt - this is semantic validity, not storage
	ValidUntil *time.Time
}

// NewTemporal creates a Temporal with creation time set to now.
func NewTemporal() *Temporal {
	now := time.Now()
	return &Temporal{
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// NewTemporalWithTTL creates a Temporal with TTL.
func NewTemporalWithTTL(ttl time.Duration) *Temporal {
	now := time.Now()
	expiresAt := now.Add(ttl)
	return &Temporal{
		CreatedAt: now,
		UpdatedAt: now,
		ExpiresAt: &expiresAt,
	}
}

// IsExpired checks if the document has expired.
func (t *Temporal) IsExpired() bool {
	if t == nil || t.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*t.ExpiresAt)
}

// IsValid checks if the document is currently valid (within ValidFrom/ValidUntil range).
func (t *Temporal) IsValid() bool {
	if t == nil {
		return true
	}

	now := time.Now()

	if t.ValidFrom != nil && now.Before(*t.ValidFrom) {
		return false
	}

	if t.ValidUntil != nil && now.After(*t.ValidUntil) {
		return false
	}

	return true
}

// SetExpiry sets the expiration time relative to now.
func (t *Temporal) SetExpiry(ttl time.Duration) {
	expiresAt := time.Now().Add(ttl)
	t.ExpiresAt = &expiresAt
}

// Touch updates the UpdatedAt timestamp to now.
func (t *Temporal) Touch() {
	t.UpdatedAt = time.Now()
}

// TimestampValue represents a timestamp that can be stored and queried.
// This is a helper type for stats and results.
type TimestampValue struct {
	time.Time
}

// NewTimestamp creates a TimestampValue from a time.Time.
func NewTimestamp(t time.Time) *TimestampValue {
	return &TimestampValue{t}
}

// Validate validates a document before storage.
// This performs comprehensive validation including:
//   - ID format and length
//   - Content presence and validity
//   - Embedding dimensions and values
//   - Metadata key safety
//   - Temporal constraints
func Validate(doc *Document) error {
	if doc == nil {
		return fmt.Errorf("document cannot be nil")
	}

	// Validate ID
	if err := ValidateID(doc.ID); err != nil {
		return fmt.Errorf("invalid document ID: %w", err)
	}

	// Validate Content
	if doc.Content == nil {
		return fmt.Errorf("document content cannot be nil")
	}
	if err := ValidateContent(doc.Content); err != nil {
		return fmt.Errorf("invalid content: %w", err)
	}

	// Validate Embedding (if present)
	if doc.Embedding != nil {
		if err := ValidateEmbedding(doc.Embedding); err != nil {
			return fmt.Errorf("invalid embedding: %w", err)
		}
	}

	// Validate Metadata keys
	for key := range doc.Metadata {
		if err := ValidateMetadataKey(key); err != nil {
			return fmt.Errorf("invalid metadata key %q: %w", key, err)
		}
	}

	// Validate Tags
	for i, tag := range doc.Tags {
		if err := ValidateTag(tag); err != nil {
			return fmt.Errorf("invalid tag at index %d: %w", i, err)
		}
	}

	// Validate Temporal (if present)
	if doc.Temporal != nil {
		if err := ValidateTemporal(doc.Temporal); err != nil {
			return fmt.Errorf("invalid temporal: %w", err)
		}
	}

	// Validate Scope (if present)
	if doc.Scope != nil {
		if err := ValidateScope(doc.Scope); err != nil {
			return fmt.Errorf("invalid scope: %w", err)
		}
	}

	return nil
}

// ValidateID validates a document ID.
func ValidateID(id string) error {
	if id == "" {
		return fmt.Errorf("ID cannot be empty")
	}
	if len(id) > 512 {
		return fmt.Errorf("ID too long: maximum 512 characters, got %d", len(id))
	}

	// Disallow path traversal
	if id == "." || id == ".." {
		return fmt.Errorf("ID cannot be '.' or '..'")
	}

	// Check for unsafe characters
	for i, r := range id {
		if r < 0x20 || r == 0x7F { // Control characters
			return fmt.Errorf("ID contains control character at position %d", i)
		}
		if r == '/' || r == '\\' || r == 0 {
			return fmt.Errorf("ID contains forbidden character at position %d", i)
		}
	}

	return nil
}

// ValidateContent validates document content.
func ValidateContent(c *Content) error {
	if c == nil {
		return fmt.Errorf("content cannot be nil")
	}

	switch c.Type {
	case ContentTypeText:
		if c.Text == "" && len(c.Chunks) == 0 {
			return fmt.Errorf("text content cannot be empty")
		}
	case ContentTypeImage, ContentTypeAudio, ContentTypeVideo:
		if len(c.Data) == 0 && c.URL == "" {
			return fmt.Errorf("%s content must have either data or URL", c.Type)
		}
		if c.MimeType == "" {
			return fmt.Errorf("%s content must specify MIME type", c.Type)
		}
	default:
		return fmt.Errorf("unknown content type: %s", c.Type)
	}

	return nil
}

// ValidateEmbedding validates an embedding vector.
func ValidateEmbedding(e *Embedding) error {
	if e == nil {
		return fmt.Errorf("embedding cannot be nil")
	}

	if len(e.Vector) == 0 {
		return fmt.Errorf("embedding vector cannot be empty")
	}

	if e.Dimensions != len(e.Vector) {
		return fmt.Errorf("embedding dimensions mismatch: declared %d, actual %d", e.Dimensions, len(e.Vector))
	}

	// Check for invalid float values
	for i, val := range e.Vector {
		if isNaN32(val) || isInf32(val) {
			return fmt.Errorf("embedding contains invalid value at index %d: %f", i, val)
		}
	}

	return nil
}

// ValidateMetadataKey validates a metadata key.
func ValidateMetadataKey(key string) error {
	if key == "" {
		return fmt.Errorf("metadata key cannot be empty")
	}
	if len(key) > 256 {
		return fmt.Errorf("metadata key too long: maximum 256 characters, got %d", len(key))
	}

	// Disallow control characters and injection-prone characters
	for i, r := range key {
		if r < 0x20 || r == 0x7F {
			return fmt.Errorf("metadata key contains control character at position %d", i)
		}
		if r == '$' || r == '.' {
			return fmt.Errorf("metadata key contains forbidden character '%c' at position %d", r, i)
		}
	}

	return nil
}

// ValidateTag validates a tag.
func ValidateTag(tag string) error {
	if tag == "" {
		return fmt.Errorf("tag cannot be empty")
	}
	if len(tag) > 128 {
		return fmt.Errorf("tag too long: maximum 128 characters, got %d", len(tag))
	}

	// Tags should be alphanumeric with hyphens and underscores
	for i, r := range tag {
		if !isAlphanumeric(r) && r != '-' && r != '_' && r != ':' {
			return fmt.Errorf("tag contains invalid character at position %d: %c", i, r)
		}
	}

	return nil
}

// ValidateTemporal validates temporal information.
func ValidateTemporal(t *Temporal) error {
	if t == nil {
		return nil
	}

	// CreatedAt should not be zero
	if t.CreatedAt.IsZero() {
		return fmt.Errorf("CreatedAt cannot be zero")
	}

	// UpdatedAt should not be before CreatedAt
	if !t.UpdatedAt.IsZero() && t.UpdatedAt.Before(t.CreatedAt) {
		return fmt.Errorf("UpdatedAt cannot be before CreatedAt")
	}

	// ValidUntil should not be before ValidFrom
	if t.ValidFrom != nil && t.ValidUntil != nil && t.ValidUntil.Before(*t.ValidFrom) {
		return fmt.Errorf("ValidUntil cannot be before ValidFrom")
	}

	return nil
}

// ValidateScope validates scope information.
func ValidateScope(s *Scope) error {
	if s == nil {
		return nil
	}

	// Validate each scope field length
	if len(s.Tenant) > 256 {
		return fmt.Errorf("tenant too long: maximum 256 characters, got %d", len(s.Tenant))
	}
	if len(s.User) > 256 {
		return fmt.Errorf("user too long: maximum 256 characters, got %d", len(s.User))
	}
	if len(s.Session) > 256 {
		return fmt.Errorf("session too long: maximum 256 characters, got %d", len(s.Session))
	}
	if len(s.Agent) > 256 {
		return fmt.Errorf("agent too long: maximum 256 characters, got %d", len(s.Agent))
	}
	if len(s.Thread) > 256 {
		return fmt.Errorf("thread too long: maximum 256 characters, got %d", len(s.Thread))
	}

	// Validate custom scope keys
	for key := range s.Custom {
		if err := ValidateMetadataKey(key); err != nil {
			return fmt.Errorf("invalid custom scope key %q: %w", key, err)
		}
	}

	return nil
}

// Helper functions

func isNaN32(f float32) bool {
	return f != f
}

func isInf32(f float32) bool {
	const maxFloat32 = 3.40282346638528859811704183484516925440e+38
	return f > maxFloat32 || f < -maxFloat32
}

func isAlphanumeric(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}
