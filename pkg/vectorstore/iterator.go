package vectorstore

import (
	"io"
)

// sliceIterator implements ResultIterator for in-memory slices.
// This is a reference implementation that providers can use for simple cases.
type sliceIterator struct {
	matches []*Match
	index   int
	err     error
}

// NewSliceIterator creates a ResultIterator from a slice of matches.
// This is a helper for providers that materialize all results in memory.
func NewSliceIterator(matches []*Match) ResultIterator {
	return &sliceIterator{
		matches: matches,
		index:   -1,
	}
}

// Next advances to the next result.
func (it *sliceIterator) Next() bool {
	if it.err != nil {
		return false
	}

	it.index++
	return it.index < len(it.matches)
}

// Match returns the current match.
func (it *sliceIterator) Match() *Match {
	if it.index < 0 || it.index >= len(it.matches) {
		return nil
	}
	return it.matches[it.index]
}

// Err returns any error that occurred.
func (it *sliceIterator) Err() error {
	return it.err
}

// Close releases resources.
func (it *sliceIterator) Close() error {
	it.matches = nil
	return nil
}

// Ensure sliceIterator implements ResultIterator and io.Closer
var _ ResultIterator = (*sliceIterator)(nil)
var _ io.Closer = (*sliceIterator)(nil)

// channelIterator implements ResultIterator for streaming from a channel.
// This is useful for providers that stream results from a database cursor.
type channelIterator struct {
	ch      <-chan *Match
	errCh   <-chan error
	current *Match
	err     error
	done    bool
}

// NewChannelIterator creates a ResultIterator from channels.
// The match channel should be closed when done.
// The error channel receives at most one error.
func NewChannelIterator(matches <-chan *Match, errs <-chan error) ResultIterator {
	return &channelIterator{
		ch:    matches,
		errCh: errs,
	}
}

// Next advances to the next result.
func (it *channelIterator) Next() bool {
	if it.done || it.err != nil {
		return false
	}

	// Check for errors first
	select {
	case err := <-it.errCh:
		if err != nil {
			it.err = err
			it.done = true
			return false
		}
	default:
	}

	// Try to receive next match
	match, ok := <-it.ch
	if !ok {
		it.done = true
		// Check for final error
		select {
		case err := <-it.errCh:
			it.err = err
		default:
		}
		return false
	}

	it.current = match
	return true
}

// Match returns the current match.
func (it *channelIterator) Match() *Match {
	return it.current
}

// Err returns any error that occurred.
func (it *channelIterator) Err() error {
	return it.err
}

// Close releases resources.
func (it *channelIterator) Close() error {
	it.done = true
	it.current = nil

	// Drain remaining matches to avoid goroutine leaks
	for range it.ch {
		// Discard
	}

	return nil
}

// Ensure channelIterator implements ResultIterator and io.Closer
var _ ResultIterator = (*channelIterator)(nil)
var _ io.Closer = (*channelIterator)(nil)

// errorIterator implements ResultIterator that immediately returns an error.
// This is useful for propagating errors from query setup.
type errorIterator struct {
	err error
}

// NewErrorIterator creates a ResultIterator that returns an error.
func NewErrorIterator(err error) ResultIterator {
	return &errorIterator{err: err}
}

// Next always returns false.
func (it *errorIterator) Next() bool {
	return false
}

// Match always returns nil.
func (it *errorIterator) Match() *Match {
	return nil
}

// Err returns the error.
func (it *errorIterator) Err() error {
	return it.err
}

// Close does nothing.
func (it *errorIterator) Close() error {
	return nil
}

// Ensure errorIterator implements ResultIterator and io.Closer
var _ ResultIterator = (*errorIterator)(nil)
var _ io.Closer = (*errorIterator)(nil)

// emptyIterator implements ResultIterator with no results.
type emptyIterator struct{}

// NewEmptyIterator creates a ResultIterator with no results.
func NewEmptyIterator() ResultIterator {
	return &emptyIterator{}
}

// Next always returns false.
func (it *emptyIterator) Next() bool {
	return false
}

// Match always returns nil.
func (it *emptyIterator) Match() *Match {
	return nil
}

// Err always returns nil.
func (it *emptyIterator) Err() error {
	return nil
}

// Close does nothing.
func (it *emptyIterator) Close() error {
	return nil
}

// Ensure emptyIterator implements ResultIterator and io.Closer
var _ ResultIterator = (*emptyIterator)(nil)
var _ io.Closer = (*emptyIterator)(nil)

// CollectAll collects all results from an iterator into a slice.
// The iterator is closed after collection.
//
// This is a convenience function for cases where you want to materialize
// all results. Use with caution on large result sets.
//
// Example:
//
//	iter, err := coll.QueryStream(ctx, query)
//	if err != nil {
//	    return err
//	}
//	matches, err := vectorstore.CollectAll(iter)
//	if err != nil {
//	    return err
//	}
func CollectAll(iter ResultIterator) ([]*Match, error) {
	defer func() { _ = iter.Close() }()

	var matches []*Match
	for iter.Next() {
		matches = append(matches, iter.Match())
	}

	if err := iter.Err(); err != nil {
		return nil, err
	}

	return matches, nil
}

// CollectN collects up to N results from an iterator.
// The iterator is NOT closed (caller should close it).
//
// Example:
//
//	iter, err := coll.QueryStream(ctx, query)
//	if err != nil {
//	    return err
//	}
//	defer iter.Close()
//
//	// Get first 10 results
//	matches, err := vectorstore.CollectN(iter, 10)
func CollectN(iter ResultIterator, n int) ([]*Match, error) {
	matches := make([]*Match, 0, n)

	for i := 0; i < n && iter.Next(); i++ {
		matches = append(matches, iter.Match())
	}

	if err := iter.Err(); err != nil {
		return nil, err
	}

	return matches, nil
}

// ForEach applies a function to each result in an iterator.
// The iterator is closed after iteration.
//
// Example:
//
//	iter, err := coll.QueryStream(ctx, query)
//	if err != nil {
//	    return err
//	}
//	err = vectorstore.ForEach(iter, func(match *Match) error {
//	    fmt.Printf("Score: %.4f, ID: %s\n", match.Score, match.Document.ID)
//	    return nil
//	})
func ForEach(iter ResultIterator, fn func(*Match) error) error {
	defer func() { _ = iter.Close() }()

	for iter.Next() {
		if err := fn(iter.Match()); err != nil {
			return err
		}
	}

	return iter.Err()
}

// FilterIterator applies a predicate to filter results from an iterator.
// Returns a new iterator with only matching results.
//
// Example:
//
//	iter, err := coll.QueryStream(ctx, query)
//	if err != nil {
//	    return err
//	}
//	defer iter.Close()
//
//	// Filter for high-scoring results
//	filtered := vectorstore.FilterIterator(iter, func(m *Match) bool {
//	    return m.Score >= 0.8
//	})
//	defer filtered.Close()
func FilterIterator(iter ResultIterator, predicate func(*Match) bool) ResultIterator {
	return &filterIterator{
		source:    iter,
		predicate: predicate,
	}
}

// filterIterator wraps an iterator with a filter predicate.
type filterIterator struct {
	source    ResultIterator
	predicate func(*Match) bool
	current   *Match
}

// Next advances to the next matching result.
func (it *filterIterator) Next() bool {
	for it.source.Next() {
		match := it.source.Match()
		if it.predicate(match) {
			it.current = match
			return true
		}
	}
	return false
}

// Match returns the current match.
func (it *filterIterator) Match() *Match {
	return it.current
}

// Err returns any error from the source iterator.
func (it *filterIterator) Err() error {
	return it.source.Err()
}

// Close closes the source iterator.
func (it *filterIterator) Close() error {
	return it.source.Close()
}

// Ensure filterIterator implements ResultIterator
var _ ResultIterator = (*filterIterator)(nil)

// MapIterator applies a transformation function to each result.
// Returns a new iterator with transformed results.
//
// Example:
//
//	iter, err := coll.QueryStream(ctx, query)
//	if err != nil {
//	    return err
//	}
//	defer iter.Close()
//
//	// Boost scores
//	boosted := vectorstore.MapIterator(iter, func(m *Match) *Match {
//	    m.Score *= 1.2
//	    if m.Score > 1.0 {
//	        m.Score = 1.0
//	    }
//	    return m
//	})
//	defer boosted.Close()
func MapIterator(iter ResultIterator, fn func(*Match) *Match) ResultIterator {
	return &mapIterator{
		source: iter,
		fn:     fn,
	}
}

// mapIterator wraps an iterator with a transformation function.
type mapIterator struct {
	source ResultIterator
	fn     func(*Match) *Match
}

// Next advances to the next result.
func (it *mapIterator) Next() bool {
	return it.source.Next()
}

// Match returns the transformed match.
func (it *mapIterator) Match() *Match {
	match := it.source.Match()
	if match == nil {
		return nil
	}
	return it.fn(match)
}

// Err returns any error from the source iterator.
func (it *mapIterator) Err() error {
	return it.source.Err()
}

// Close closes the source iterator.
func (it *mapIterator) Close() error {
	return it.source.Close()
}

// Ensure mapIterator implements ResultIterator
var _ ResultIterator = (*mapIterator)(nil)
