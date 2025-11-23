package vectorstore

import (
	"fmt"
	"time"
)

// QueryResult contains the results of a similarity search query.
type QueryResult struct {
	// Matches are the matching documents with their scores
	Matches []*Match

	// Total is the total number of matches (before limit/offset)
	// Useful for pagination
	Total int64

	// Offset is the offset that was applied
	Offset int

	// Limit is the limit that was applied
	Limit int

	// Timing contains query execution timing information
	Timing *QueryTiming

	// Explain contains query execution details (if requested)
	Explain *QueryExplain
}

// Match represents a single search result with similarity score.
type Match struct {
	// Document is the matched document
	Document *Document

	// Score is the similarity score (higher is more similar)
	// For cosine similarity: -1 (opposite) to 1 (identical)
	// For euclidean: normalized to 0-1 range
	Score float32

	// Distance is the raw distance metric (optional)
	// The interpretation depends on the distance metric used
	Distance float32

	// Rank is the result rank (1-based)
	// Useful for hybrid ranking scenarios
	Rank int
}

// QueryTiming contains timing information for a query.
type QueryTiming struct {
	// Total is the total query execution time
	Total time.Duration

	// VectorSearch is the time spent on vector similarity search
	VectorSearch time.Duration

	// FilterApplication is the time spent applying filters
	FilterApplication time.Duration

	// Retrieval is the time spent retrieving full documents
	Retrieval time.Duration

	// Scoring is the time spent calculating similarity scores
	Scoring time.Duration
}

// QueryExplain contains detailed query execution information.
// This is useful for debugging and optimization.
type QueryExplain struct {
	// Strategy describes the query execution strategy
	// Examples: "brute_force", "hnsw_index", "ivf_index"
	Strategy string

	// IndexUsed indicates which index was used (if any)
	IndexUsed string

	// ScannedDocuments is the number of documents scanned
	ScannedDocuments int64

	// FilteredDocuments is the number of documents after filtering
	FilteredDocuments int64

	// VectorComparisons is the number of vector similarity comparisons
	VectorComparisons int64

	// CacheHit indicates if results were served from cache
	CacheHit bool

	// Steps contains detailed execution steps
	Steps []ExplainStep
}

// ExplainStep represents a single step in query execution.
type ExplainStep struct {
	// Name is the step name
	Name string

	// Duration is how long this step took
	Duration time.Duration

	// Details contains step-specific information
	Details map[string]any
}

// UpsertResult contains the results of an upsert operation.
type UpsertResult struct {
	// Inserted is the number of new documents inserted
	Inserted int64

	// Updated is the number of existing documents updated
	Updated int64

	// Failed is the number of documents that failed validation/insertion
	Failed int64

	// FailedIDs contains the IDs of documents that failed (if any)
	FailedIDs []string

	// Errors contains errors for failed documents (parallel to FailedIDs)
	Errors []error

	// Timing contains operation timing information
	Timing *OperationTiming

	// Deduplicated is the number of documents that were deduplicated
	// Only set if deduplication is enabled
	Deduplicated int64

	// DeduplicatedIDs contains the IDs of deduplicated documents
	DeduplicatedIDs []string
}

// DeleteResult contains the results of a delete operation.
type DeleteResult struct {
	// Deleted is the number of documents actually deleted
	Deleted int64

	// NotFound is the number of IDs that were not found
	NotFound int64

	// NotFoundIDs contains the IDs that were not found
	NotFoundIDs []string

	// Timing contains operation timing information
	Timing *OperationTiming
}

// OperationTiming contains timing information for CRUD operations.
type OperationTiming struct {
	// Total is the total operation time
	Total time.Duration

	// Validation is the time spent validating documents
	Validation time.Duration

	// Storage is the time spent writing to storage
	Storage time.Duration

	// IndexUpdate is the time spent updating indexes
	IndexUpdate time.Duration
}

// Success returns true if all documents were successfully upserted.
func (r *UpsertResult) Success() bool {
	return r.Failed == 0
}

// Success returns true if at least one document was deleted.
func (r *DeleteResult) Success() bool {
	return r.Deleted > 0
}

// PartialSuccess returns true if some documents succeeded but some failed.
func (r *UpsertResult) PartialSuccess() bool {
	return (r.Inserted+r.Updated) > 0 && r.Failed > 0
}

// TotalProcessed returns the total number of documents processed.
func (r *UpsertResult) TotalProcessed() int64 {
	return r.Inserted + r.Updated + r.Failed
}

// HasMatches returns true if the query returned any matches.
func (r *QueryResult) HasMatches() bool {
	return len(r.Matches) > 0
}

// TopMatch returns the highest scoring match, or nil if no matches.
func (r *QueryResult) TopMatch() *Match {
	if len(r.Matches) == 0 {
		return nil
	}
	return r.Matches[0]
}

// HasMore returns true if there are more results beyond the current page.
func (r *QueryResult) HasMore() bool {
	return r.Total > int64(r.Offset+len(r.Matches))
}

// NextOffset returns the offset for the next page.
func (r *QueryResult) NextOffset() int {
	return r.Offset + len(r.Matches)
}

// PrevOffset returns the offset for the previous page.
func (r *QueryResult) PrevOffset() int {
	offset := r.Offset - r.Limit
	if offset < 0 {
		return 0
	}
	return offset
}

// Documents returns just the documents from matches (without scores).
func (r *QueryResult) Documents() []*Document {
	docs := make([]*Document, len(r.Matches))
	for i, match := range r.Matches {
		docs[i] = match.Document
	}
	return docs
}

// Scores returns just the scores from matches.
func (r *QueryResult) Scores() []float32 {
	scores := make([]float32, len(r.Matches))
	for i, match := range r.Matches {
		scores[i] = match.Score
	}
	return scores
}

// IDs returns just the document IDs from matches.
func (r *QueryResult) IDs() []string {
	ids := make([]string, len(r.Matches))
	for i, match := range r.Matches {
		ids[i] = match.Document.ID
	}
	return ids
}

// MatchByID finds a match by document ID.
func (r *QueryResult) MatchByID(id string) *Match {
	for _, match := range r.Matches {
		if match.Document.ID == id {
			return match
		}
	}
	return nil
}

// FilterByScore returns matches with score >= minScore.
func (r *QueryResult) FilterByScore(minScore float32) []*Match {
	var filtered []*Match
	for _, match := range r.Matches {
		if match.Score >= minScore {
			filtered = append(filtered, match)
		}
	}
	return filtered
}

// FilterByTag returns matches with a specific tag.
func (r *QueryResult) FilterByTag(tag string) []*Match {
	var filtered []*Match
	for _, match := range r.Matches {
		for _, t := range match.Document.Tags {
			if t == tag {
				filtered = append(filtered, match)
				break
			}
		}
	}
	return filtered
}

// GroupByTag groups matches by tag.
// Each match may appear in multiple groups if it has multiple tags.
func (r *QueryResult) GroupByTag() map[string][]*Match {
	groups := make(map[string][]*Match)
	for _, match := range r.Matches {
		for _, tag := range match.Document.Tags {
			groups[tag] = append(groups[tag], match)
		}
	}
	return groups
}

// Empty returns true if there are no matches.
func (r *QueryResult) Empty() bool {
	return len(r.Matches) == 0
}

// Count returns the number of matches in this result.
func (r *QueryResult) Count() int {
	return len(r.Matches)
}

// AvgScore returns the average similarity score across all matches.
func (r *QueryResult) AvgScore() float32 {
	if len(r.Matches) == 0 {
		return 0
	}

	var sum float32
	for _, match := range r.Matches {
		sum += match.Score
	}
	return sum / float32(len(r.Matches))
}

// MaxScore returns the highest similarity score.
func (r *QueryResult) MaxScore() float32 {
	if len(r.Matches) == 0 {
		return 0
	}

	max := r.Matches[0].Score
	for _, match := range r.Matches[1:] {
		if match.Score > max {
			max = match.Score
		}
	}
	return max
}

// MinScore returns the lowest similarity score.
func (r *QueryResult) MinScore() float32 {
	if len(r.Matches) == 0 {
		return 0
	}

	min := r.Matches[0].Score
	for _, match := range r.Matches[1:] {
		if match.Score < min {
			min = match.Score
		}
	}
	return min
}

// Slice returns a slice of matches [start:end].
func (r *QueryResult) Slice(start, end int) []*Match {
	if start < 0 {
		start = 0
	}
	if end > len(r.Matches) {
		end = len(r.Matches)
	}
	if start >= end {
		return nil
	}
	return r.Matches[start:end]
}

// First returns the first N matches.
func (r *QueryResult) First(n int) []*Match {
	if n > len(r.Matches) {
		n = len(r.Matches)
	}
	return r.Matches[:n]
}

// Last returns the last N matches.
func (r *QueryResult) Last(n int) []*Match {
	if n > len(r.Matches) {
		n = len(r.Matches)
	}
	return r.Matches[len(r.Matches)-n:]
}

// ExplainString returns a human-readable explanation of query execution.
func (e *QueryExplain) ExplainString() string {
	if e == nil {
		return "No explain information available"
	}

	s := "Query Execution Plan:\n"
	s += "  Strategy: " + e.Strategy + "\n"
	if e.IndexUsed != "" {
		s += "  Index: " + e.IndexUsed + "\n"
	}
	s += "  Documents Scanned: " + formatInt64(e.ScannedDocuments) + "\n"
	s += "  Documents Filtered: " + formatInt64(e.FilteredDocuments) + "\n"
	s += "  Vector Comparisons: " + formatInt64(e.VectorComparisons) + "\n"
	if e.CacheHit {
		s += "  Cache: HIT\n"
	}

	if len(e.Steps) > 0 {
		s += "\nExecution Steps:\n"
		for _, step := range e.Steps {
			s += "  - " + step.Name + " (" + step.Duration.String() + ")\n"
		}
	}

	return s
}

// Helper function to format int64
func formatInt64(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1000000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	if n < 1000000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	}
	return fmt.Sprintf("%.1fB", float64(n)/1000000000)
}
