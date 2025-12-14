package aggregation

import (
	"fmt"
	"sort"
	"strings"
)

// VotingInput represents a single input for voting
type VotingInput struct {
	Source     string         // Agent name that produced this input
	Content    string         // The actual content to vote on
	Confidence float64        // Confidence score (0-1)
	Metadata   map[string]any // Additional metadata
}

// VotingResult contains the voting outcome
type VotingResult struct {
	SelectedContent string         // The winning content
	Agreement       float64        // Level of agreement (0-1)
	Strategy        string         // Strategy used
	Explanation     string         // How the decision was made
	Votes           map[string]int // Vote counts per content
}

// MajorityVote implements simple majority voting
// Returns the content that appears most frequently
func MajorityVote(inputs []VotingInput) (*VotingResult, error) {
	if len(inputs) == 0 {
		return nil, fmt.Errorf("no inputs to vote on")
	}

	// Single input case
	if len(inputs) == 1 {
		return &VotingResult{
			SelectedContent: inputs[0].Content,
			Agreement:       1.0,
			Strategy:        "majority",
			Explanation:     "Single input - unanimous agreement",
			Votes:           map[string]int{inputs[0].Content: 1},
		}, nil
	}

	// Count votes for each unique content
	votes := make(map[string]int)
	contentMap := make(map[string]string)     // Normalized -> Original content
	sources := make(map[string][]string)      // Track which sources voted for what
	confidenceSum := make(map[string]float64) // Sum of confidences for each content

	for _, input := range inputs {
		normalized := normalizeContent(input.Content)
		votes[normalized]++
		contentMap[normalized] = input.Content
		sources[normalized] = append(sources[normalized], input.Source)
		confidenceSum[normalized] += input.Confidence
	}

	// Find the content with most votes
	var maxVotes int
	var winner string
	var tieCount int

	for content, count := range votes {
		if count > maxVotes {
			maxVotes = count
			winner = content
			tieCount = 1
		} else if count == maxVotes {
			tieCount++
		}
	}

	// Handle ties - break by average confidence
	if tieCount > 1 {
		var tied []string
		for content, count := range votes {
			if count == maxVotes {
				tied = append(tied, content)
			}
		}

		// Pick the option with highest average confidence
		var bestAvgConfidence float64
		for _, content := range tied {
			avgConfidence := confidenceSum[content] / float64(votes[content])
			if avgConfidence > bestAvgConfidence {
				bestAvgConfidence = avgConfidence
				winner = content
			}
		}
	}

	// Calculate agreement level
	agreement := float64(maxVotes) / float64(len(inputs))

	// Build explanation
	explanation := fmt.Sprintf("Majority vote: %d/%d agents agreed on selected content. "+
		"Sources: %s",
		maxVotes, len(inputs), strings.Join(sources[winner], ", "))

	if tieCount > 1 {
		avgConf := confidenceSum[winner] / float64(votes[winner])
		explanation += fmt.Sprintf(" (broke %d-way tie by confidence: %.2f avg)", tieCount, avgConf)
	}

	return &VotingResult{
		SelectedContent: contentMap[winner],
		Agreement:       agreement,
		Strategy:        "majority",
		Explanation:     explanation,
		Votes:           votes,
	}, nil
}

// UnanimousVote requires all inputs to agree
// Returns error if there's any disagreement
func UnanimousVote(inputs []VotingInput) (*VotingResult, error) {
	if len(inputs) == 0 {
		return nil, fmt.Errorf("no inputs to vote on")
	}

	// Single input is always unanimous
	if len(inputs) == 1 {
		return &VotingResult{
			SelectedContent: inputs[0].Content,
			Agreement:       1.0,
			Strategy:        "unanimous",
			Explanation:     "Single input - unanimous agreement",
			Votes:           map[string]int{inputs[0].Content: 1},
		}, nil
	}

	// Check if all inputs have the same content
	firstContent := normalizeContent(inputs[0].Content)
	votes := make(map[string]int)
	votes[firstContent] = 1

	for i := 1; i < len(inputs); i++ {
		normalized := normalizeContent(inputs[i].Content)
		votes[normalized]++
		if normalized != firstContent {
			// Disagreement found
			return nil, fmt.Errorf("unanimous vote failed: %s disagrees with %s (found %d different opinions)",
				inputs[i].Source, inputs[0].Source, len(votes))
		}
	}

	// All agreed
	sources := make([]string, len(inputs))
	for i, input := range inputs {
		sources[i] = input.Source
	}

	return &VotingResult{
		SelectedContent: inputs[0].Content,
		Agreement:       1.0,
		Strategy:        "unanimous",
		Explanation:     fmt.Sprintf("All %d agents agreed unanimously. Sources: %s", len(inputs), strings.Join(sources, ", ")),
		Votes:           map[string]int{inputs[0].Content: len(inputs)},
	}, nil
}

// WeightedVote uses confidence weights
// Content with highest weighted score wins
func WeightedVote(inputs []VotingInput) (*VotingResult, error) {
	if len(inputs) == 0 {
		return nil, fmt.Errorf("no inputs to vote on")
	}

	// Single input case
	if len(inputs) == 1 {
		return &VotingResult{
			SelectedContent: inputs[0].Content,
			Agreement:       1.0,
			Strategy:        "weighted",
			Explanation:     fmt.Sprintf("Single input with confidence %.2f", inputs[0].Confidence),
			Votes:           map[string]int{inputs[0].Content: 1},
		}, nil
	}

	// Calculate weighted scores for each unique content
	weightedScores := make(map[string]float64)
	contentMap := make(map[string]string)
	sources := make(map[string][]string)
	totalWeight := 0.0

	for _, input := range inputs {
		normalized := normalizeContent(input.Content)
		weight := input.Confidence
		if weight == 0 {
			weight = 0.5 // Default weight if not specified
		}

		weightedScores[normalized] += weight
		totalWeight += weight
		contentMap[normalized] = input.Content
		sources[normalized] = append(sources[normalized], fmt.Sprintf("%s(%.2f)", input.Source, weight))
	}

	// Find content with highest weighted score
	var maxScore float64
	var winner string
	var tieCount int

	for content, score := range weightedScores {
		if score > maxScore {
			maxScore = score
			winner = content
			tieCount = 1
		} else if score == maxScore {
			tieCount++
		}
	}

	// Handle ties deterministically - select first alphabetically
	if tieCount > 1 {
		var tied []string
		for content, score := range weightedScores {
			if score == maxScore {
				tied = append(tied, content)
			}
		}
		sort.Strings(tied)
		winner = tied[0]
	}

	// Calculate agreement as proportion of total weight
	agreement := maxScore / totalWeight
	if agreement > 1.0 {
		agreement = 1.0
	}

	// Build explanation
	explanation := fmt.Sprintf("Weighted vote: selected content has weighted score %.2f/%.2f (%.0f%% of total weight). "+
		"Sources: %s",
		maxScore, totalWeight, agreement*100, strings.Join(sources[winner], ", "))

	if tieCount > 1 {
		explanation += " (broke tie deterministically)"
	}

	// Convert weighted scores to vote counts for the result
	votes := make(map[string]int)
	for content := range weightedScores {
		votes[content] = len(sources[content])
	}

	return &VotingResult{
		SelectedContent: contentMap[winner],
		Agreement:       agreement,
		Strategy:        "weighted",
		Explanation:     explanation,
		Votes:           votes,
	}, nil
}

// ConfidenceVote selects input with highest confidence
// Ignores content, just picks most confident source
func ConfidenceVote(inputs []VotingInput) (*VotingResult, error) {
	if len(inputs) == 0 {
		return nil, fmt.Errorf("no inputs to vote on")
	}

	// Single input case
	if len(inputs) == 1 {
		return &VotingResult{
			SelectedContent: inputs[0].Content,
			Agreement:       1.0,
			Strategy:        "confidence",
			Explanation:     fmt.Sprintf("Single input from %s with confidence %.2f", inputs[0].Source, inputs[0].Confidence),
			Votes:           map[string]int{inputs[0].Content: 1},
		}, nil
	}

	// Find input with highest confidence
	maxConfidence := -1.0
	var bestInput *VotingInput
	var tieCount int
	var tied []*VotingInput

	for i := range inputs {
		input := &inputs[i]
		confidence := input.Confidence
		if confidence == 0 {
			confidence = 0.5 // Default confidence
		}

		if confidence > maxConfidence {
			maxConfidence = confidence
			bestInput = input
			tied = []*VotingInput{input}
			tieCount = 1
		} else if confidence == maxConfidence {
			tieCount++
			tied = append(tied, input)
		}
	}

	// Handle ties deterministically - select first source alphabetically
	if tieCount > 1 {
		sort.Slice(tied, func(i, j int) bool {
			return tied[i].Source < tied[j].Source
		})
		bestInput = tied[0]
	}

	if bestInput == nil {
		return nil, fmt.Errorf("no confident input found")
	}

	// Calculate agreement based on confidence distribution
	var totalConfidence float64
	for i := range inputs {
		conf := inputs[i].Confidence
		if conf == 0 {
			conf = 0.5
		}
		totalConfidence += conf
	}

	agreement := maxConfidence / totalConfidence * float64(len(inputs))
	if agreement > 1.0 {
		agreement = 1.0
	}

	// Build explanation
	explanation := fmt.Sprintf("Confidence vote: selected input from %s with highest confidence %.2f",
		bestInput.Source, maxConfidence)

	if tieCount > 1 {
		var tiedSources []string
		for _, t := range tied {
			tiedSources = append(tiedSources, t.Source)
		}
		explanation += fmt.Sprintf(" (broke %d-way tie among %s deterministically)",
			tieCount, strings.Join(tiedSources, ", "))
	}

	votes := make(map[string]int)
	for _, input := range inputs {
		normalized := normalizeContent(input.Content)
		votes[normalized]++
	}

	return &VotingResult{
		SelectedContent: bestInput.Content,
		Agreement:       agreement,
		Strategy:        "confidence",
		Explanation:     explanation,
		Votes:           votes,
	}, nil
}

// normalizeContent normalizes content for comparison
// This makes voting deterministic by handling whitespace consistently
func normalizeContent(content string) string {
	// Trim whitespace and convert to lowercase for comparison
	normalized := strings.TrimSpace(content)
	normalized = strings.ToLower(normalized)
	// Normalize multiple spaces to single space
	normalized = strings.Join(strings.Fields(normalized), " ")
	return normalized
}
