# Deterministic Voting Tests - Aixgo v0.1.2

This directory contains comprehensive tests for the deterministic voting aggregation feature.

## Test Coverage

### Unit Tests (`voting_test.go`)

#### 1. MajorityVote Tests
- **Clear majority (3/5 agree)**: Verifies that when 3 out of 5 agents agree, the majority option wins
- **Tie scenarios**: Tests tie-breaking by confidence (2/2, 1/1/1)
- **Empty inputs**: Ensures error is returned for empty input
- **Single input**: Validates single input returns that input
- **All agree**: Perfect consensus returns the agreed content
- **Majority with variations**: Exact string matching is used

#### 2. UnanimousVote Tests
- **All agree (success)**: Unanimous decision when all agents agree
- **One disagrees (failure)**: Returns error when any agent disagrees
- **Empty inputs (error)**: Error for empty input
- **Single input unanimous**: Single vote is considered unanimous
- **Case-sensitive disagreement**: Case differences count as disagreement
- **Whitespace disagreement**: Whitespace matters in comparison

#### 3. WeightedVote Tests
- **Weighted consensus**: Higher weighted option wins
- **High confidence wins over count**: Balances count and confidence
- **Equal weights**: Majority wins when weights are equal
- **Zero confidence ignored**: Zero confidence votes don't count

#### 4. ConfidenceVote Tests
- **Highest confidence wins**: Agent with highest confidence is selected
- **Tie in confidence**: First one wins in tie
- **Ignore content, only confidence**: Content length doesn't matter
- **Zero confidence loses**: Even low confidence beats zero

#### 5. Deterministic Reproducibility Tests
- **100 iterations per strategy**: Runs each voting function 100 times
- **Identical outputs**: Asserts exact same results every time
- **All 4 strategies tested**: MajorityVote, UnanimousVote, WeightedVote, ConfidenceVote

#### 6. Edge Cases
- **Extremely long content** (100KB): Handles large strings
- **Special characters**: Newlines, tabs, carriage returns
- **Unicode content**: Emoji and international characters
- **Negative confidence**: Invalid negative values
- **Confidence over 1.0**: Invalid high values
- **Empty string content**: Empty strings are valid
- **Many inputs (100 agents)**: Scales to large input sets

#### 7. Metadata Tests
- **Metadata preservation**: Ensures metadata is correctly populated
- **Voting statistics**: Verifies metadata contains voting info

#### 8. Benchmarks
- **BenchmarkMajorityVote**: Performance measurement
- **BenchmarkUnanimousVote**: Performance measurement
- **BenchmarkWeightedVote**: Performance measurement
- **BenchmarkConfidenceVote**: Performance measurement

### Integration Tests (`aggregator_test.go`)

#### 1. Voting Strategy Tests
- **TestAggregator_VotingMajority**: Full majority voting workflow
  - Clear majority selection
  - Tie broken by confidence
  - Single input handling
  - Zero token usage verification

- **TestAggregator_VotingUnanimous**: Unanimous voting workflow
  - All agents agree (success)
  - One agent disagrees (failure)
  - Perfect consensus level
  - Zero token usage

- **TestAggregator_VotingWeighted**: Weighted voting workflow
  - Weighted by confidence
  - High confidence single vote
  - Zero confidence ignored
  - Zero token usage

- **TestAggregator_VotingConfidence**: Confidence-based voting
  - Highest confidence wins
  - Content length ignored
  - Zero token usage
  - Confidence metadata

#### 2. Deterministic Verification Tests
- **TestAggregator_NoLLMCallsForDeterministic**:
  - All 4 voting strategies verified
  - Mock provider assertions
  - Zero token usage for all

- **TestAggregator_DeterministicReproducibility**:
  - 50 iterations per strategy
  - Identical results guaranteed
  - Zero token usage every time

#### 3. Edge Case Tests
- **TestAggregator_DeterministicEdgeCases**:
  - Empty input buffer
  - All same content (perfect consensus)
  - All different content
  - Missing confidence values
  - Extremely long content (100KB)

#### 4. Backwards Compatibility Tests
- **TestAggregator_ExistingStrategiesUnchanged**:
  - Tests all 5 existing strategies (consensus, weighted, semantic, hierarchical, rag_based)
  - Verifies they still call LLM
  - Confirms token usage > 0

- **TestAggregator_DefaultStrategy**:
  - Verifies default is still consensus
  - Confirms LLM usage for default

### Full Workflow Tests (`aggregator_integration_test.go`)

#### 1. YAML Configuration Tests
- **TestAggregator_YAMLConfig_DeterministicStrategy**:
  - Load voting_majority from config
  - Verify deterministic behavior
  - Zero token usage

#### 2. Mixed Strategy Tests
- **TestAggregator_MixedStrategies**:
  - Switching from LLM to deterministic
  - Parallel usage without interference
  - No cross-contamination

#### 3. Performance Tests
- **TestAggregator_PerformanceComparison**:
  - Benchmark deterministic (100 iterations)
  - Benchmark LLM with simulated latency (10 iterations)
  - Assert deterministic < 1ms per iteration
  - Assert LLM ~100ms per iteration
  - Verify 100x+ speed improvement

#### 4. Complete Workflow Tests
- **TestAggregator_FullWorkflow**:
  - voting_majority complete workflow
  - voting_unanimous complete workflow
  - Input buffering
  - Result aggregation
  - Source tracking

#### 5. Concurrency Tests
- **TestAggregator_ConcurrentDeterministic**:
  - 100 concurrent aggregations
  - Thread safety verification
  - Identical results across goroutines

#### 6. Error Handling Tests
- **TestAggregator_ErrorHandling**:
  - Invalid strategy
  - Empty content
  - Nil inputs

#### 7. Metadata Tests
- **TestAggregator_MetadataPreservation**:
  - Source metadata tracking
  - Timestamp preservation
  - Custom metadata fields

## Test Coverage Goals

- **Unit test coverage**: 90%+ for voting functions
- **Integration coverage**: 80%+ for aggregator strategies
- **Edge cases**: All identified edge cases tested
- **Reproducibility**: All deterministic strategies tested 50-100 times
- **Performance**: Benchmarks comparing LLM vs deterministic

## Running Tests

### Run all tests
```bash
cd /Users/charlesgreen/go/src/github.com/aixgo-dev/aixgo
go test ./internal/aggregation/... -v
go test ./agents/... -v -run "Aggregator"
```

### Run specific test suites
```bash
# Voting unit tests only
go test ./internal/aggregation/... -v -run "TestMajorityVote|TestUnanimousVote|TestWeightedVote|TestConfidenceVote"

# Deterministic reproducibility
go test ./internal/aggregation/... -v -run "TestDeterministicReproducibility"

# Integration tests
go test ./agents/... -v -run "TestAggregator_.*Integration"

# Performance comparison
go test ./agents/... -v -run "TestAggregator_PerformanceComparison"
```

### Run benchmarks
```bash
go test ./internal/aggregation/... -bench=. -benchmem
```

### Generate coverage report
```bash
go test ./internal/aggregation/... -coverprofile=coverage_voting.out
go test ./agents/... -run "Aggregator" -coverprofile=coverage_aggregator.out
go tool cover -html=coverage_voting.out -o coverage_voting.html
go tool cover -html=coverage_aggregator.out -o coverage_aggregator.html
```

## Expected Test Results

### All Tests Should Pass
- ✅ 0 LLM calls for deterministic strategies
- ✅ Identical results across 50-100 iterations
- ✅ Deterministic < 1ms per operation
- ✅ LLM ~100ms per operation (when simulated)
- ✅ 100x+ performance improvement
- ✅ Thread-safe concurrent execution
- ✅ Existing strategies still work (backwards compatible)

### Test Statistics
- **Total test functions**: 40+
- **Total test cases**: 100+
- **Reproducibility iterations**: 5000+ (50-100 per strategy)
- **Concurrency tests**: 100 goroutines
- **Performance iterations**: 100+ deterministic, 10+ LLM

## Key Verification Points

1. **Zero LLM Calls**: All deterministic strategies must use zero tokens
2. **Reproducibility**: Same inputs always produce same outputs
3. **Performance**: Deterministic is 100x+ faster than LLM
4. **Backwards Compatibility**: Existing strategies unchanged
5. **Thread Safety**: Concurrent execution produces identical results
6. **Edge Cases**: All edge cases handled gracefully

## Test-Driven Development

These tests are designed to guide the implementation of the voting functions:

1. **Red Phase**: Tests currently fail (functions not implemented)
2. **Green Phase**: Implement functions to pass tests
3. **Refactor Phase**: Optimize while keeping tests green

The tests define the exact expected behavior and serve as:
- Specification documentation
- Implementation guide
- Regression protection
- Performance baseline
