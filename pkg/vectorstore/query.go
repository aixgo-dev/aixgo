package vectorstore

import (
	"fmt"
	"time"
)

// Query defines parameters for similarity search.
// It supports vector similarity, metadata filters, temporal constraints,
// scope filters, pagination, and more.
type Query struct {
	// Embedding is the query vector for similarity search.
	// If nil, performs a pure metadata/filter query without vector similarity.
	Embedding *Embedding

	// Filters specifies conditions that documents must match.
	// Can be combined using And(), Or(), Not() for complex queries.
	Filters Filter

	// Limit is the maximum number of results to return.
	// Default: 10. Maximum: 10000.
	Limit int

	// Offset is the number of results to skip (for pagination).
	// Default: 0.
	Offset int

	// MinScore is the minimum similarity score (0.0-1.0).
	// Documents with lower scores are excluded.
	// Default: 0 (no minimum).
	MinScore float32

	// Metric specifies how to calculate vector similarity.
	// Default: Cosine similarity.
	Metric DistanceMetric

	// IncludeEmbeddings controls whether to return embeddings in results.
	// Default: false (embeddings are large and often not needed).
	IncludeEmbeddings bool

	// IncludeContent controls whether to return document content in results.
	// Default: true. Set to false to only get metadata/scores.
	IncludeContent bool

	// SortBy specifies additional sorting criteria (after similarity).
	// Useful for hybrid ranking (e.g., similarity + recency).
	SortBy []SortBy

	// Explain requests query execution details (for debugging/optimization).
	// Default: false.
	Explain bool
}

// NewQuery creates a query with an embedding vector.
func NewQuery(embedding *Embedding) *Query {
	return &Query{
		Embedding:      embedding,
		Limit:          10,
		IncludeContent: true,
	}
}

// NewFilterQuery creates a filter-only query (no vector similarity).
func NewFilterQuery(filters Filter) *Query {
	return &Query{
		Filters:        filters,
		Limit:          10,
		IncludeContent: true,
	}
}

// Validate validates a query.
func (q *Query) Validate() error {
	if q == nil {
		return fmt.Errorf("query cannot be nil")
	}

	// Either embedding or filters must be specified
	if q.Embedding == nil && q.Filters == nil {
		return fmt.Errorf("query must have either embedding or filters")
	}

	// Validate embedding if present
	if q.Embedding != nil {
		if err := ValidateEmbedding(q.Embedding); err != nil {
			return fmt.Errorf("invalid query embedding: %w", err)
		}
	}

	// Validate limit
	if q.Limit < 1 {
		return fmt.Errorf("limit must be at least 1, got %d", q.Limit)
	}
	if q.Limit > 10000 {
		return fmt.Errorf("limit cannot exceed 10000, got %d", q.Limit)
	}

	// Validate offset
	if q.Offset < 0 {
		return fmt.Errorf("offset cannot be negative, got %d", q.Offset)
	}

	// Validate MinScore
	if q.MinScore < 0 || q.MinScore > 1 {
		return fmt.Errorf("MinScore must be between 0 and 1, got %f", q.MinScore)
	}

	// Validate metric
	if q.Metric != "" {
		switch q.Metric {
		case DistanceMetricCosine, DistanceMetricEuclidean, DistanceMetricDotProduct:
			// Valid
		default:
			return fmt.Errorf("invalid distance metric: %s", q.Metric)
		}
	}

	return nil
}

// Filter represents a condition for filtering documents.
// Filters can be combined using And(), Or(), Not().
type Filter interface {
	// filterMarker is a private method to prevent external implementations
	filterMarker()
}

// Composite filters

// andFilter represents an AND combination of filters.
type andFilter struct {
	filters []Filter
}

func (f *andFilter) filterMarker() {}

// And combines multiple filters with AND logic (all must match).
func And(filters ...Filter) Filter {
	return &andFilter{filters: filters}
}

// orFilter represents an OR combination of filters.
type orFilter struct {
	filters []Filter
}

func (f *orFilter) filterMarker() {}

// Or combines multiple filters with OR logic (at least one must match).
func Or(filters ...Filter) Filter {
	return &orFilter{filters: filters}
}

// notFilter represents a NOT filter (negation).
type notFilter struct {
	filter Filter
}

func (f *notFilter) filterMarker() {}

// Not negates a filter.
func Not(filter Filter) Filter {
	return &notFilter{filter: filter}
}

// Field filters

// fieldFilter represents a field-based filter.
type fieldFilter struct {
	field    string
	operator FilterOperator
	value    any
}

func (f *fieldFilter) filterMarker() {}

// FieldFilter creates a filter on a metadata field.
func FieldFilter(field string, operator FilterOperator, value any) Filter {
	return &fieldFilter{
		field:    field,
		operator: operator,
		value:    value,
	}
}

// FilterOperator represents a comparison operator.
type FilterOperator string

const (
	// OpEqual checks for equality (==)
	OpEqual FilterOperator = "eq"

	// OpNotEqual checks for inequality (!=)
	OpNotEqual FilterOperator = "ne"

	// OpGreaterThan checks if field > value
	OpGreaterThan FilterOperator = "gt"

	// OpGreaterThanOrEqual checks if field >= value
	OpGreaterThanOrEqual FilterOperator = "gte"

	// OpLessThan checks if field < value
	OpLessThan FilterOperator = "lt"

	// OpLessThanOrEqual checks if field <= value
	OpLessThanOrEqual FilterOperator = "lte"

	// OpIn checks if field is in a set of values
	OpIn FilterOperator = "in"

	// OpNotIn checks if field is not in a set of values
	OpNotIn FilterOperator = "nin"

	// OpContains checks if a string field contains a substring
	OpContains FilterOperator = "contains"

	// OpStartsWith checks if a string field starts with a prefix
	OpStartsWith FilterOperator = "startswith"

	// OpEndsWith checks if a string field ends with a suffix
	OpEndsWith FilterOperator = "endswith"

	// OpExists checks if a field exists (value is ignored)
	OpExists FilterOperator = "exists"

	// OpNotExists checks if a field does not exist (value is ignored)
	OpNotExists FilterOperator = "notexists"
)

// Convenience filter constructors

// Eq creates an equality filter.
func Eq(field string, value any) Filter {
	return FieldFilter(field, OpEqual, value)
}

// Ne creates an inequality filter.
func Ne(field string, value any) Filter {
	return FieldFilter(field, OpNotEqual, value)
}

// Gt creates a greater-than filter.
func Gt(field string, value any) Filter {
	return FieldFilter(field, OpGreaterThan, value)
}

// Gte creates a greater-than-or-equal filter.
func Gte(field string, value any) Filter {
	return FieldFilter(field, OpGreaterThanOrEqual, value)
}

// Lt creates a less-than filter.
func Lt(field string, value any) Filter {
	return FieldFilter(field, OpLessThan, value)
}

// Lte creates a less-than-or-equal filter.
func Lte(field string, value any) Filter {
	return FieldFilter(field, OpLessThanOrEqual, value)
}

// In creates an in-set filter.
func In(field string, values ...any) Filter {
	return FieldFilter(field, OpIn, values)
}

// NotIn creates a not-in-set filter.
func NotIn(field string, values ...any) Filter {
	return FieldFilter(field, OpNotIn, values)
}

// Contains creates a substring filter.
func Contains(field string, substring string) Filter {
	return FieldFilter(field, OpContains, substring)
}

// StartsWith creates a prefix filter.
func StartsWith(field string, prefix string) Filter {
	return FieldFilter(field, OpStartsWith, prefix)
}

// EndsWith creates a suffix filter.
func EndsWith(field string, suffix string) Filter {
	return FieldFilter(field, OpEndsWith, suffix)
}

// Exists creates an existence filter.
func Exists(field string) Filter {
	return FieldFilter(field, OpExists, true)
}

// NotExists creates a non-existence filter.
func NotExists(field string) Filter {
	return FieldFilter(field, OpNotExists, true)
}

// Tag filters

// tagFilter represents a tag-based filter.
type tagFilter struct {
	tag string
}

func (f *tagFilter) filterMarker() {}

// TagFilter creates a filter that matches documents with a specific tag.
func TagFilter(tag string) Filter {
	return &tagFilter{tag: tag}
}

// TagsFilter creates a filter that matches documents with all specified tags.
func TagsFilter(tags ...string) Filter {
	filters := make([]Filter, len(tags))
	for i, tag := range tags {
		filters[i] = TagFilter(tag)
	}
	return And(filters...)
}

// AnyTagFilter creates a filter that matches documents with any of the specified tags.
func AnyTagFilter(tags ...string) Filter {
	filters := make([]Filter, len(tags))
	for i, tag := range tags {
		filters[i] = TagFilter(tag)
	}
	return Or(filters...)
}

// Scope filters

// scopeFilter represents a scope-based filter.
type scopeFilter struct {
	scope *Scope
}

func (f *scopeFilter) filterMarker() {}

// ScopeFilter creates a filter based on scope.
// Documents must match all non-empty scope fields.
func ScopeFilter(scope *Scope) Filter {
	return &scopeFilter{scope: scope}
}

// TenantFilter creates a filter for a specific tenant.
func TenantFilter(tenant string) Filter {
	return ScopeFilter(&Scope{Tenant: tenant})
}

// UserFilter creates a filter for a specific user.
func UserFilter(user string) Filter {
	return ScopeFilter(&Scope{User: user})
}

// SessionFilter creates a filter for a specific session.
func SessionFilter(session string) Filter {
	return ScopeFilter(&Scope{Session: session})
}

// Temporal filters

// timeFilter represents a time-based filter.
type timeFilter struct {
	field    TimeField
	operator FilterOperator
	value    time.Time
}

func (f *timeFilter) filterMarker() {}

// TimeField represents a temporal field to filter on.
type TimeField string

const (
	// TimeFieldCreatedAt filters on document creation time
	TimeFieldCreatedAt TimeField = "created_at"

	// TimeFieldUpdatedAt filters on document update time
	TimeFieldUpdatedAt TimeField = "updated_at"

	// TimeFieldExpiresAt filters on document expiration time
	TimeFieldExpiresAt TimeField = "expires_at"

	// TimeFieldEventTime filters on event time
	TimeFieldEventTime TimeField = "event_time"

	// TimeFieldValidFrom filters on validity start time
	TimeFieldValidFrom TimeField = "valid_from"

	// TimeFieldValidUntil filters on validity end time
	TimeFieldValidUntil TimeField = "valid_until"
)

// TimeFilter creates a time-based filter.
func TimeFilter(field TimeField, operator FilterOperator, value time.Time) Filter {
	return &timeFilter{
		field:    field,
		operator: operator,
		value:    value,
	}
}

// CreatedAfter filters documents created after a time.
func CreatedAfter(t time.Time) Filter {
	return TimeFilter(TimeFieldCreatedAt, OpGreaterThan, t)
}

// CreatedBefore filters documents created before a time.
func CreatedBefore(t time.Time) Filter {
	return TimeFilter(TimeFieldCreatedAt, OpLessThan, t)
}

// UpdatedAfter filters documents updated after a time.
func UpdatedAfter(t time.Time) Filter {
	return TimeFilter(TimeFieldUpdatedAt, OpGreaterThan, t)
}

// UpdatedBefore filters documents updated before a time.
func UpdatedBefore(t time.Time) Filter {
	return TimeFilter(TimeFieldUpdatedAt, OpLessThan, t)
}

// ExpiresAfter filters documents that expire after a time.
func ExpiresAfter(t time.Time) Filter {
	return TimeFilter(TimeFieldExpiresAt, OpGreaterThan, t)
}

// ExpiresBefore filters documents that expire before a time.
func ExpiresBefore(t time.Time) Filter {
	return TimeFilter(TimeFieldExpiresAt, OpLessThan, t)
}

// NotExpired filters documents that have not expired yet.
func NotExpired() Filter {
	return ExpiresAfter(time.Now())
}

// Expired filters documents that have expired.
func Expired() Filter {
	return ExpiresBefore(time.Now())
}

// Score filters

// scoreFilter represents a similarity score filter.
type scoreFilter struct {
	operator FilterOperator
	value    float32
}

func (f *scoreFilter) filterMarker() {}

// ScoreFilter creates a filter based on similarity score.
// Only applies during vector similarity queries.
func ScoreFilter(operator FilterOperator, value float32) Filter {
	return &scoreFilter{
		operator: operator,
		value:    value,
	}
}

// ScoreAbove filters results with score > threshold.
func ScoreAbove(threshold float32) Filter {
	return ScoreFilter(OpGreaterThan, threshold)
}

// ScoreBelow filters results with score < threshold.
func ScoreBelow(threshold float32) Filter {
	return ScoreFilter(OpLessThan, threshold)
}

// ScoreAtLeast filters results with score >= threshold.
func ScoreAtLeast(threshold float32) Filter {
	return ScoreFilter(OpGreaterThanOrEqual, threshold)
}

// SortBy specifies a field to sort by.
type SortBy struct {
	// Field is the field name to sort by
	// Can be a metadata field, score, or temporal field
	Field string

	// Descending indicates descending order (default is ascending)
	Descending bool
}

// SortByScore creates a sort by similarity score (descending by default).
func SortByScore() SortBy {
	return SortBy{Field: "score", Descending: true}
}

// SortByCreatedAt creates a sort by creation time.
func SortByCreatedAt(descending bool) SortBy {
	return SortBy{Field: "created_at", Descending: descending}
}

// SortByUpdatedAt creates a sort by update time.
func SortByUpdatedAt(descending bool) SortBy {
	return SortBy{Field: "updated_at", Descending: descending}
}

// SortByField creates a sort by metadata field.
func SortByField(field string, descending bool) SortBy {
	return SortBy{Field: field, Descending: descending}
}

// DistanceMetric represents the method for calculating vector similarity.
type DistanceMetric string

const (
	// DistanceMetricCosine calculates cosine similarity (default)
	// Range: -1 (opposite) to 1 (identical)
	// Best for: Most text embeddings (normalized vectors)
	DistanceMetricCosine DistanceMetric = "cosine"

	// DistanceMetricEuclidean calculates Euclidean (L2) distance
	// Range: 0 (identical) to infinity (different)
	// Best for: When magnitude matters
	DistanceMetricEuclidean DistanceMetric = "euclidean"

	// DistanceMetricDotProduct calculates dot product similarity
	// Range: -infinity to +infinity
	// Best for: Normalized vectors, faster than cosine
	DistanceMetricDotProduct DistanceMetric = "dot_product"

	// DistanceMetricManhattan calculates Manhattan (L1) distance
	// Range: 0 (identical) to infinity (different)
	// Best for: High-dimensional sparse vectors
	DistanceMetricManhattan DistanceMetric = "manhattan"

	// DistanceMetricHamming calculates Hamming distance (for binary vectors)
	// Range: 0 (identical) to vector_length (completely different)
	// Best for: Binary embeddings
	DistanceMetricHamming DistanceMetric = "hamming"
)

// GetFilters returns all filters in the composite filter.
// This is used internally by providers to decompose complex filters.
func GetFilters(f Filter) []Filter {
	switch v := f.(type) {
	case *andFilter:
		return v.filters
	case *orFilter:
		return v.filters
	default:
		return []Filter{f}
	}
}

// GetFieldFilter extracts field, operator, and value from a field filter.
// Returns empty values if not a field filter.
func GetFieldFilter(f Filter) (field string, op FilterOperator, value any, ok bool) {
	if ff, isField := f.(*fieldFilter); isField {
		return ff.field, ff.operator, ff.value, true
	}
	return "", "", nil, false
}

// GetTagFilter extracts the tag from a tag filter.
// Returns empty string if not a tag filter.
func GetTagFilter(f Filter) (tag string, ok bool) {
	if tf, isTag := f.(*tagFilter); isTag {
		return tf.tag, true
	}
	return "", false
}

// GetScopeFilter extracts the scope from a scope filter.
// Returns nil if not a scope filter.
func GetScopeFilter(f Filter) (scope *Scope, ok bool) {
	if sf, isScope := f.(*scopeFilter); isScope {
		return sf.scope, true
	}
	return nil, false
}

// GetTimeFilter extracts field, operator, and value from a time filter.
// Returns zero values if not a time filter.
func GetTimeFilter(f Filter) (field TimeField, op FilterOperator, value time.Time, ok bool) {
	if tf, isTime := f.(*timeFilter); isTime {
		return tf.field, tf.operator, tf.value, true
	}
	return "", "", time.Time{}, false
}

// GetScoreFilter extracts operator and value from a score filter.
// Returns zero values if not a score filter.
func GetScoreFilter(f Filter) (op FilterOperator, value float32, ok bool) {
	if sf, isScore := f.(*scoreFilter); isScore {
		return sf.operator, sf.value, true
	}
	return "", 0, false
}

// IsAndFilter checks if the filter is an AND composite.
func IsAndFilter(f Filter) bool {
	_, ok := f.(*andFilter)
	return ok
}

// IsOrFilter checks if the filter is an OR composite.
func IsOrFilter(f Filter) bool {
	_, ok := f.(*orFilter)
	return ok
}

// IsNotFilter checks if the filter is a NOT filter.
func IsNotFilter(f Filter) bool {
	_, ok := f.(*notFilter)
	return ok
}

// GetNotFilter extracts the inner filter from a NOT filter.
func GetNotFilter(f Filter) (inner Filter, ok bool) {
	if nf, isNot := f.(*notFilter); isNot {
		return nf.filter, true
	}
	return nil, false
}
