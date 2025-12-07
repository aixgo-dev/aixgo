package aggregation

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test type aliases for convenience - map to actual types
type VoteInput = VotingInput

// TestMajorityVote tests the majority voting strategy
func TestMajorityVote(t *testing.T) {
	tests := []struct {
		name           string
		inputs         []VoteInput
		expectedResult string
		expectError    bool
		description    string
	}{
		{
			name: "clear_majority_3_of_5",
			inputs: []VoteInput{
				{Content: "Solution A", Confidence: 0.8, Source: "agent1"},
				{Content: "Solution A", Confidence: 0.9, Source: "agent2"},
				{Content: "Solution A", Confidence: 0.7, Source: "agent3"},
				{Content: "Solution B", Confidence: 0.85, Source: "agent4"},
				{Content: "Solution C", Confidence: 0.75, Source: "agent5"},
			},
			expectedResult: "solution a", // Normalized content
			expectError:    false,
			description:    "3 out of 5 agents agree on Solution A - clear majority",
		},
		{
			name: "tie_scenario_2_2",
			inputs: []VoteInput{
				{Content: "Option X", Confidence: 0.9, Source: "agent1"},
				{Content: "Option X", Confidence: 0.8, Source: "agent2"},
				{Content: "Option Y", Confidence: 0.95, Source: "agent3"},
				{Content: "Option Y", Confidence: 0.85, Source: "agent4"},
			},
			expectedResult: "option y", // Tie broken by confidence (Y has avg 0.9 vs X's 0.85)
			expectError:    false,
			description:    "Majority vote breaks ties by average confidence",
		},
		{
			name: "tie_scenario_1_1_1",
			inputs: []VoteInput{
				{Content: "Alpha", Confidence: 0.7, Source: "agent1"},
				{Content: "Beta", Confidence: 0.8, Source: "agent2"},
				{Content: "Gamma", Confidence: 0.9, Source: "agent3"},
			},
			expectedResult: "gamma", // Highest confidence wins three-way tie
			expectError:    false,
			description:    "Three-way tie - highest confidence wins",
		},
		{
			name:           "empty_inputs",
			inputs:         []VoteInput{},
			expectedResult: "",
			expectError:    true,
			description:    "Empty input should return error",
		},
		{
			name: "single_input",
			inputs: []VoteInput{
				{Content: "Only option", Confidence: 0.8, Source: "agent1"},
			},
			expectedResult: "Only option",
			expectError:    false,
			description:    "Single input returns that input",
		},
		{
			name: "all_agree",
			inputs: []VoteInput{
				{Content: "Consensus", Confidence: 0.8, Source: "agent1"},
				{Content: "Consensus", Confidence: 0.9, Source: "agent2"},
				{Content: "Consensus", Confidence: 0.85, Source: "agent3"},
				{Content: "Consensus", Confidence: 0.95, Source: "agent4"},
			},
			expectedResult: "Consensus",
			expectError:    false,
			description:    "Perfect agreement - all agents agree",
		},
		{
			name: "majority_with_variations",
			inputs: []VoteInput{
				{Content: "Answer A", Confidence: 0.8, Source: "agent1"},
				{Content: "Answer A with details", Confidence: 0.9, Source: "agent2"}, // Similar but not exact
				{Content: "Answer B", Confidence: 0.85, Source: "agent3"},
			},
			expectedResult: "Answer A", // Exact match wins
			expectError:    false,
			description:    "Majority voting uses exact string matching",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := MajorityVote(tt.inputs)

			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				require.NoError(t, err, tt.description)
				assert.Contains(t, strings.ToLower(result.SelectedContent), strings.ToLower(tt.expectedResult), tt.description)
				assert.Greater(t, result.Agreement, 0.0, "Agreement should be positive")
				assert.LessOrEqual(t, result.Agreement, 1.0, "Agreement should not exceed 1.0")
			}
		})
	}
}

// TestUnanimousVote tests the unanimous voting strategy
func TestUnanimousVote(t *testing.T) {
	tests := []struct {
		name           string
		inputs         []VoteInput
		expectedResult string
		expectError    bool
		description    string
	}{
		{
			name: "all_agree_success",
			inputs: []VoteInput{
				{Content: "Unanimous decision", Confidence: 0.8, Source: "agent1"},
				{Content: "Unanimous decision", Confidence: 0.9, Source: "agent2"},
				{Content: "Unanimous decision", Confidence: 0.85, Source: "agent3"},
			},
			expectedResult: "Unanimous decision",
			expectError:    false,
			description:    "All agents agree - unanimous vote succeeds",
		},
		{
			name: "one_disagrees_failure",
			inputs: []VoteInput{
				{Content: "Option A", Confidence: 0.8, Source: "agent1"},
				{Content: "Option A", Confidence: 0.9, Source: "agent2"},
				{Content: "Option B", Confidence: 0.85, Source: "agent3"},
			},
			expectedResult: "",
			expectError:    true,
			description:    "One agent disagrees - unanimous vote fails",
		},
		{
			name:           "empty_inputs_error",
			inputs:         []VoteInput{},
			expectedResult: "",
			expectError:    true,
			description:    "Empty input should return error",
		},
		{
			name: "single_input_unanimous",
			inputs: []VoteInput{
				{Content: "Single vote", Confidence: 0.9, Source: "agent1"},
			},
			expectedResult: "Single vote",
			expectError:    false,
			description:    "Single input is unanimous",
		},
		{
			name: "case_insensitive_agreement",
			inputs: []VoteInput{
				{Content: "Result", Confidence: 0.8, Source: "agent1"},
				{Content: "result", Confidence: 0.9, Source: "agent2"}, // Different case but normalized
			},
			expectedResult: "Result", // First content returned
			expectError:    false,
			description:    "Implementation normalizes case - treats as same",
		},
		{
			name: "whitespace_normalized",
			inputs: []VoteInput{
				{Content: "Result", Confidence: 0.8, Source: "agent1"},
				{Content: "Result ", Confidence: 0.9, Source: "agent2"}, // Trailing space normalized
			},
			expectedResult: "Result",
			expectError:    false,
			description:    "Implementation normalizes whitespace - treats as same",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := UnanimousVote(tt.inputs)

			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				require.NoError(t, err, tt.description)
				assert.Equal(t, tt.expectedResult, result.SelectedContent, tt.description)
			}
		})
	}
}

// TestWeightedVote tests the weighted voting strategy
func TestWeightedVote(t *testing.T) {
	tests := []struct {
		name           string
		inputs         []VoteInput
		expectedResult string
		expectError    bool
		description    string
	}{
		{
			name: "weighted_consensus",
			inputs: []VoteInput{
				{Content: "Option A", Confidence: 0.9, Source: "agent1"}, // 0.9 weight
				{Content: "Option A", Confidence: 0.8, Source: "agent2"}, // 0.8 weight
				{Content: "Option B", Confidence: 0.5, Source: "agent3"}, // 0.5 weight
			},
			expectedResult: "Option A", // Total weight: 1.7 vs 0.5
			expectError:    false,
			description:    "Higher weighted option wins",
		},
		{
			name: "high_confidence_wins_over_count",
			inputs: []VoteInput{
				{Content: "Option A", Confidence: 0.6, Source: "agent1"},
				{Content: "Option A", Confidence: 0.6, Source: "agent2"},
				{Content: "Option B", Confidence: 0.99, Source: "agent3"}, // Single high confidence
			},
			expectedResult: "Option A", // 1.2 total weight vs 0.99
			expectError:    false,
			description:    "Weighted voting considers both count and confidence",
		},
		{
			name: "equal_weights_majority_wins",
			inputs: []VoteInput{
				{Content: "Option X", Confidence: 0.8, Source: "agent1"},
				{Content: "Option X", Confidence: 0.8, Source: "agent2"},
				{Content: "Option Y", Confidence: 0.8, Source: "agent3"},
			},
			expectedResult: "Option X", // 1.6 vs 0.8
			expectError:    false,
			description:    "Equal weights - majority wins",
		},
		{
			name: "zero_confidence_ignored",
			inputs: []VoteInput{
				{Content: "Option A", Confidence: 0.0, Source: "agent1"}, // Should be ignored
				{Content: "Option B", Confidence: 0.8, Source: "agent2"},
			},
			expectedResult: "Option B",
			expectError:    false,
			description:    "Zero confidence votes should be ignored",
		},
		{
			name:           "empty_inputs",
			inputs:         []VoteInput{},
			expectedResult: "",
			expectError:    true,
			description:    "Empty input should return error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := WeightedVote(tt.inputs)

			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				require.NoError(t, err, tt.description)
				assert.Equal(t, tt.expectedResult, result.SelectedContent, tt.description)
			}
		})
	}
}

// TestConfidenceVote tests the confidence-based voting strategy
func TestConfidenceVote(t *testing.T) {
	tests := []struct {
		name           string
		inputs         []VoteInput
		expectedResult string
		expectError    bool
		description    string
	}{
		{
			name: "highest_confidence_wins",
			inputs: []VoteInput{
				{Content: "Option A", Confidence: 0.7, Source: "agent1"},
				{Content: "Option B", Confidence: 0.95, Source: "agent2"},
				{Content: "Option C", Confidence: 0.6, Source: "agent3"},
			},
			expectedResult: "Option B",
			expectError:    false,
			description:    "Agent with highest confidence wins",
		},
		{
			name: "tie_in_confidence",
			inputs: []VoteInput{
				{Content: "Option X", Confidence: 0.9, Source: "agent1"},
				{Content: "Option Y", Confidence: 0.9, Source: "agent2"},
			},
			expectedResult: "Option X", // First one wins in tie
			expectError:    false,
			description:    "Tie in confidence - first one wins",
		},
		{
			name: "ignore_content_only_confidence",
			inputs: []VoteInput{
				{Content: "Long detailed answer with lots of information", Confidence: 0.6, Source: "agent1"},
				{Content: "Short", Confidence: 0.99, Source: "agent2"},
			},
			expectedResult: "Short",
			expectError:    false,
			description:    "Content length doesn't matter, only confidence",
		},
		{
			name:           "empty_inputs",
			inputs:         []VoteInput{},
			expectedResult: "",
			expectError:    true,
			description:    "Empty input should return error",
		},
		{
			name: "single_input",
			inputs: []VoteInput{
				{Content: "Only choice", Confidence: 0.5, Source: "agent1"},
			},
			expectedResult: "Only choice",
			expectError:    false,
			description:    "Single input returns that input",
		},
		{
			name: "zero_confidence_gets_default",
			inputs: []VoteInput{
				{Content: "Option A", Confidence: 0.0, Source: "agent1"}, // Gets 0.5 default
				{Content: "Option B", Confidence: 0.1, Source: "agent2"},
			},
			expectedResult: "Option A", // 0.5 default > 0.1
			expectError:    false,
			description:    "Zero confidence gets default value of 0.5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ConfidenceVote(tt.inputs)

			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				require.NoError(t, err, tt.description)
				assert.Equal(t, tt.expectedResult, result.SelectedContent, tt.description)
			}
		})
	}
}

// TestDeterministicReproducibility verifies deterministic behavior
func TestDeterministicReproducibility(t *testing.T) {
	inputs := []VoteInput{
		{Content: "Answer A", Confidence: 0.8, Source: "agent1"},
		{Content: "Answer A", Confidence: 0.7, Source: "agent2"},
		{Content: "Answer B", Confidence: 0.9, Source: "agent3"},
		{Content: "Answer C", Confidence: 0.6, Source: "agent4"},
	}

	t.Run("MajorityVote_100_iterations", func(t *testing.T) {
		var firstResult string
		for i := 0; i < 100; i++ {
			result, err := MajorityVote(inputs)
			require.NoError(t, err)

			if i == 0 {
				firstResult = result.SelectedContent
			} else {
				assert.Equal(t, firstResult, result.SelectedContent,
					"MajorityVote should produce identical results on iteration %d", i)
			}
		}
	})

	t.Run("UnanimousVote_100_iterations", func(t *testing.T) {
		unanimousInputs := []VoteInput{
			{Content: "Same", Confidence: 0.8, Source: "agent1"},
			{Content: "Same", Confidence: 0.9, Source: "agent2"},
			{Content: "Same", Confidence: 0.7, Source: "agent3"},
		}

		var firstResult string
		var firstError error
		for i := 0; i < 100; i++ {
			result, err := UnanimousVote(unanimousInputs)

			if i == 0 {
				firstResult = result.SelectedContent
				firstError = err
			} else {
				if firstError != nil {
					assert.Error(t, err, "Error state should be consistent")
				} else {
					require.NoError(t, err)
					assert.Equal(t, firstResult, result.SelectedContent,
						"UnanimousVote should produce identical results on iteration %d", i)
				}
			}
		}
	})

	t.Run("WeightedVote_100_iterations", func(t *testing.T) {
		var firstResult string
		for i := 0; i < 100; i++ {
			result, err := WeightedVote(inputs)
			require.NoError(t, err)

			if i == 0 {
				firstResult = result.SelectedContent
			} else {
				assert.Equal(t, firstResult, result.SelectedContent,
					"WeightedVote should produce identical results on iteration %d", i)
			}
		}
	})

	t.Run("ConfidenceVote_100_iterations", func(t *testing.T) {
		var firstResult string
		for i := 0; i < 100; i++ {
			result, err := ConfidenceVote(inputs)
			require.NoError(t, err)

			if i == 0 {
				firstResult = result.SelectedContent
			} else {
				assert.Equal(t, firstResult, result.SelectedContent,
					"ConfidenceVote should produce identical results on iteration %d", i)
			}
		}
	})
}

// TestEdgeCases tests edge cases across all voting functions
func TestEdgeCases(t *testing.T) {
	t.Run("extremely_long_content", func(t *testing.T) {
		longContent := string(make([]byte, 100000)) // 100KB content
		inputs := []VoteInput{
			{Content: longContent, Confidence: 0.8, Source: "agent1"},
			{Content: "Short", Confidence: 0.7, Source: "agent2"},
		}

		result, err := MajorityVote(inputs)
		require.NoError(t, err)
		assert.NotEmpty(t, result.SelectedContent)
	})

	t.Run("special_characters_in_content", func(t *testing.T) {
		inputs := []VoteInput{
			{Content: "Hello\nWorld\t\r\n", Confidence: 0.8, Source: "agent1"},
			{Content: "Hello\nWorld\t\r\n", Confidence: 0.9, Source: "agent2"},
			{Content: "Different", Confidence: 0.7, Source: "agent3"},
		}

		result, err := MajorityVote(inputs)
		require.NoError(t, err)
		assert.Equal(t, "Hello\nWorld\t\r\n", result.SelectedContent)
	})

	t.Run("unicode_content", func(t *testing.T) {
		inputs := []VoteInput{
			{Content: "你好世界 🌍", Confidence: 0.8, Source: "agent1"},
			{Content: "你好世界 🌍", Confidence: 0.9, Source: "agent2"},
		}

		result, err := UnanimousVote(inputs)
		require.NoError(t, err)
		assert.Equal(t, "你好世界 🌍", result.SelectedContent)
	})

	t.Run("negative_confidence", func(t *testing.T) {
		inputs := []VoteInput{
			{Content: "Option A", Confidence: -0.5, Source: "agent1"}, // Invalid
			{Content: "Option B", Confidence: 0.8, Source: "agent2"},
		}

		// Should either error or treat negative as 0
		result, err := WeightedVote(inputs)
		if err == nil {
			assert.Equal(t, "Option B", result.SelectedContent)
		}
	})

	t.Run("confidence_over_1.0", func(t *testing.T) {
		inputs := []VoteInput{
			{Content: "Option A", Confidence: 1.5, Source: "agent1"}, // Invalid
			{Content: "Option B", Confidence: 0.8, Source: "agent2"},
		}

		// Should either error or cap at 1.0
		result, err := ConfidenceVote(inputs)
		if err == nil {
			assert.Equal(t, "Option A", result.SelectedContent) // Higher confidence wins
		}
	})

	t.Run("empty_string_content", func(t *testing.T) {
		inputs := []VoteInput{
			{Content: "", Confidence: 0.8, Source: "agent1"},
			{Content: "", Confidence: 0.9, Source: "agent2"},
		}

		result, err := UnanimousVote(inputs)
		require.NoError(t, err)
		assert.Equal(t, "", result.SelectedContent)
	})

	t.Run("many_inputs_100_agents", func(t *testing.T) {
		inputs := make([]VoteInput, 100)
		for i := 0; i < 100; i++ {
			inputs[i] = VoteInput{
				Content:    "Option A",
				Confidence: 0.8,
				Source:  string(rune('a' + (i % 26))),
			}
		}

		result, err := MajorityVote(inputs)
		require.NoError(t, err)
		assert.Equal(t, "Option A", result.SelectedContent)
	})
}

// TestVotingMetadata tests that metadata is correctly populated
func TestVotingMetadata(t *testing.T) {
	inputs := []VoteInput{
		{Content: "Option A", Confidence: 0.8, Source: "agent1"},
		{Content: "Option A", Confidence: 0.9, Source: "agent2"},
		{Content: "Option B", Confidence: 0.7, Source: "agent3"},
	}

	t.Run("MajorityVote_metadata", func(t *testing.T) {
		result, err := MajorityVote(inputs)
		require.NoError(t, err)
		assert.NotNil(t, result.Votes)
		assert.NotEmpty(t, result.Explanation)
		// VotingResult contains Votes map and Explanation
	})

	t.Run("WeightedVote_metadata", func(t *testing.T) {
		result, err := WeightedVote(inputs)
		require.NoError(t, err)
		assert.NotNil(t, result.Votes)
		assert.NotEmpty(t, result.Explanation)
		// VotingResult contains Votes map and Explanation
	})
}

// BenchmarkVotingFunctions benchmarks the voting functions
func BenchmarkMajorityVote(b *testing.B) {
	inputs := []VoteInput{
		{Content: "Option A", Confidence: 0.8, Source: "agent1"},
		{Content: "Option A", Confidence: 0.9, Source: "agent2"},
		{Content: "Option B", Confidence: 0.7, Source: "agent3"},
		{Content: "Option C", Confidence: 0.6, Source: "agent4"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = MajorityVote(inputs)
	}
}

func BenchmarkUnanimousVote(b *testing.B) {
	inputs := []VoteInput{
		{Content: "Same", Confidence: 0.8, Source: "agent1"},
		{Content: "Same", Confidence: 0.9, Source: "agent2"},
		{Content: "Same", Confidence: 0.7, Source: "agent3"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = UnanimousVote(inputs)
	}
}

func BenchmarkWeightedVote(b *testing.B) {
	inputs := []VoteInput{
		{Content: "Option A", Confidence: 0.8, Source: "agent1"},
		{Content: "Option A", Confidence: 0.9, Source: "agent2"},
		{Content: "Option B", Confidence: 0.7, Source: "agent3"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = WeightedVote(inputs)
	}
}

func BenchmarkConfidenceVote(b *testing.B) {
	inputs := []VoteInput{
		{Content: "Option A", Confidence: 0.8, Source: "agent1"},
		{Content: "Option B", Confidence: 0.95, Source: "agent2"},
		{Content: "Option C", Confidence: 0.7, Source: "agent3"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ConfidenceVote(inputs)
	}
}
