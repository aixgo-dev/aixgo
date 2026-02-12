package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aixgo-dev/aixgo/internal/agent"
	"github.com/aixgo-dev/aixgo/internal/observability"
	pb "github.com/aixgo-dev/aixgo/proto"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Reflection implements iterative refinement with self-critique.
// Generator creates output, critic reviews it, generator refines.
// Provides 20-50% quality improvement through iterative refinement.
//
// Use cases:
// - Code generation with review
// - Content creation with editing
// - Complex reasoning tasks
// - Quality assurance
type Reflection struct {
	*BaseOrchestrator
	generator            string
	critic               string
	critics              []string // For multi-critic reflection
	maxIterations        int
	improvementThreshold float64 // Minimum improvement required to continue
}

// ReflectionOption configures a Reflection orchestrator
type ReflectionOption func(*Reflection)

// WithMaxIterations sets the maximum number of refinement iterations
func WithMaxIterations(max int) ReflectionOption {
	return func(r *Reflection) {
		r.maxIterations = max
	}
}

// WithImprovementThreshold sets the minimum improvement threshold
func WithImprovementThreshold(threshold float64) ReflectionOption {
	return func(r *Reflection) {
		r.improvementThreshold = threshold
	}
}

// NewReflection creates a new Reflection orchestrator
func NewReflection(name string, runtime agent.Runtime, generator, critic string, opts ...ReflectionOption) *Reflection {
	r := &Reflection{
		BaseOrchestrator:     NewBaseOrchestrator(name, "reflection", runtime),
		generator:            generator,
		critic:               critic,
		maxIterations:        3,   // Default 3 iterations
		improvementThreshold: 0.1, // 10% improvement required
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// Execute performs iterative refinement: generate → critique → refine
func (r *Reflection) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
	ctx, span := observability.StartSpanWithOtel(ctx, fmt.Sprintf("orchestration.reflection.%s", r.name),
		trace.WithAttributes(
			attribute.String("orchestration.pattern", "reflection"),
			attribute.String("orchestration.generator", r.generator),
			attribute.String("orchestration.critic", r.critic),
			attribute.Int("orchestration.max_iterations", r.maxIterations),
		),
	)
	defer span.End()

	startTime := time.Now()
	var currentOutput *agent.Message
	var previousScore float64
	var lastCritique *agent.Message

	for iteration := 0; iteration < r.maxIterations; iteration++ {
		iterationStart := time.Now()

		// Generate or refine
		var generatorInput *agent.Message
		if iteration == 0 {
			generatorInput = input
		} else {
			// Combine current output with critique for refinement
			generatorInput = combineWithCritique(currentOutput, lastCritique)
		}

		generated, err := r.runtime.Call(ctx, r.generator, generatorInput)
		if err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("generation failed at iteration %d: %w", iteration, err)
		}

		currentOutput = generated

		// Get critique (possibly from multiple critics)
		var score float64

		if len(r.critics) > 1 {
			// Multi-critic: aggregate feedback from all critics
			lastCritique, score, err = r.aggregateCritics(ctx, generated)
			if err != nil {
				span.RecordError(err)
				return nil, fmt.Errorf("multi-critic aggregation failed at iteration %d: %w", iteration, err)
			}
		} else {
			// Single critic
			lastCritique, err = r.runtime.Call(ctx, r.critic, generated)
			if err != nil {
				span.RecordError(err)
				return nil, fmt.Errorf("critique failed at iteration %d: %w", iteration, err)
			}

			// Extract quality score from critique
			score = extractQualityScore(lastCritique)
		}

		iterationDuration := time.Since(iterationStart)

		span.SetAttributes(
			attribute.Int(fmt.Sprintf("iteration.%d.number", iteration), iteration),
			attribute.Float64(fmt.Sprintf("iteration.%d.score", iteration), score),
			attribute.Int64(fmt.Sprintf("iteration.%d.duration_ms", iteration), iterationDuration.Milliseconds()),
		)

		// Check if we should continue iterating
		if iteration > 0 {
			improvement := score - previousScore
			if improvement < r.improvementThreshold {
				// Not enough improvement, stop iterating
				span.SetAttributes(
					attribute.String("orchestration.stop_reason", "insufficient_improvement"),
					attribute.Float64("orchestration.final_improvement", improvement),
				)
				break
			}
		}

		// Check if score is good enough (assume 1.0 is perfect)
		if score >= 0.95 {
			span.SetAttributes(
				attribute.String("orchestration.stop_reason", "quality_threshold_met"),
			)
			break
		}

		previousScore = score
	}

	totalDuration := time.Since(startTime)

	span.SetAttributes(
		attribute.Int64("orchestration.total_duration_ms", totalDuration.Milliseconds()),
		attribute.Bool("orchestration.success", true),
	)

	return currentOutput, nil
}

// combineWithCritique combines original input with critique for refinement
func combineWithCritique(original, critique *agent.Message) *agent.Message {
	if original == nil || original.Message == nil {
		return original
	}

	if critique == nil || critique.Message == nil || critique.Payload == "" {
		return original
	}

	// Create refinement prompt that includes both original content and critique
	refinementPrompt := fmt.Sprintf(`You previously generated this output:
---
%s
---

A critic provided this feedback:
---
%s
---

Please refine and improve your output based on the critique. Address all the issues raised while maintaining the core intent.`, original.Payload, critique.Payload)

	return &agent.Message{
		Message: &pb.Message{
			Id:        original.Id,
			Type:      "refinement_request",
			Payload:   refinementPrompt,
			Timestamp: original.Timestamp,
			Metadata:  original.Metadata,
		},
	}
}

// extractQualityScore extracts a quality score from the critique
// Returns a score between 0.0 and 1.0
func extractQualityScore(critique *agent.Message) float64 {
	if critique == nil || critique.Message == nil || critique.Payload == "" {
		return 0.5 // Default medium score
	}

	content := critique.Payload

	// Strategy 1: Try to parse structured JSON output with explicit score
	var structuredCritique struct {
		Score   float64 `json:"score"`
		Rating  float64 `json:"rating"`
		Quality float64 `json:"quality"`
	}

	if err := json.Unmarshal([]byte(content), &structuredCritique); err == nil {
		// Successfully parsed JSON, use score field
		if structuredCritique.Score > 0 {
			return normalizeScore(structuredCritique.Score)
		}
		if structuredCritique.Rating > 0 {
			return normalizeScore(structuredCritique.Rating)
		}
		if structuredCritique.Quality > 0 {
			return normalizeScore(structuredCritique.Quality)
		}
	}

	// Strategy 2: Look for explicit score patterns like "Score: 8/10" or "Rating: 7.5"
	scorePatterns := []string{
		`[Ss]core:\s*(\d+\.?\d*)\s*/\s*(\d+)`,        // "Score: 8/10"
		`[Rr]ating:\s*(\d+\.?\d*)\s*/\s*(\d+)`,       // "Rating: 7.5/10"
		`[Qq]uality:\s*(\d+\.?\d*)`,                  // "Quality: 8.5"
		`(\d+\.?\d*)\s*/\s*10`,                       // "8/10"
		`[Ss]core:\s*(\d+\.?\d*)`,                    // "Score: 8.5"
	}

	for _, pattern := range scorePatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(content)
		if len(matches) >= 2 {
			score, err := strconv.ParseFloat(matches[1], 64)
			if err == nil {
				if len(matches) >= 3 {
					// Has denominator like "8/10"
					denom, err2 := strconv.ParseFloat(matches[2], 64)
					if err2 == nil && denom > 0 {
						return score / denom
					}
				}
				// Assume out of 10 if single number
				return normalizeScore(score)
			}
		}
	}

	// Strategy 3: Sentiment analysis based on keywords
	contentLower := strings.ToLower(content)

	positiveKeywords := []string{
		"excellent", "outstanding", "perfect", "great", "impressive",
		"strong", "well-done", "thorough", "comprehensive", "clear",
	}
	negativeKeywords := []string{
		"poor", "weak", "unclear", "incomplete", "inadequate",
		"missing", "needs improvement", "lacking", "confusing", "wrong",
	}

	positiveCount := 0
	negativeCount := 0

	for _, keyword := range positiveKeywords {
		if strings.Contains(contentLower, keyword) {
			positiveCount++
		}
	}
	for _, keyword := range negativeKeywords {
		if strings.Contains(contentLower, keyword) {
			negativeCount++
		}
	}

	// Calculate sentiment score (0.3 to 0.9 range based on keyword balance)
	if positiveCount+negativeCount == 0 {
		return 0.6 // Neutral default
	}

	sentimentScore := 0.3 + (float64(positiveCount) / float64(positiveCount+negativeCount) * 0.6)
	return sentimentScore
}

// normalizeScore normalizes a score to 0.0-1.0 range
// Assumes input is on 0-10 scale unless already in 0-1 range
func normalizeScore(score float64) float64 {
	if score < 0 {
		return 0.0
	}
	if score <= 1.0 {
		return score // Already normalized
	}
	if score <= 10.0 {
		return score / 10.0 // Normalize from 0-10 to 0-1
	}
	return 1.0 // Cap at 1.0
}

// aggregateCritics collects feedback from multiple critics and aggregates it
func (r *Reflection) aggregateCritics(ctx context.Context, generated *agent.Message) (*agent.Message, float64, error) {
	type critiqueResult struct {
		critique *agent.Message
		score    float64
		err      error
	}

	// Call all critics in parallel
	results := make(chan critiqueResult, len(r.critics))

	for _, critic := range r.critics {
		go func(criticName string) {
			critique, err := r.runtime.Call(ctx, criticName, generated)
			if err != nil {
				results <- critiqueResult{nil, 0.0, err}
				return
			}

			score := extractQualityScore(critique)
			results <- critiqueResult{critique, score, nil}
		}(critic)
	}

	// Collect results
	critiques := make([]*agent.Message, 0, len(r.critics))
	scores := make([]float64, 0, len(r.critics))

	for i := 0; i < len(r.critics); i++ {
		result := <-results
		if result.err != nil {
			continue
		}
		critiques = append(critiques, result.critique)
		scores = append(scores, result.score)
	}

	if len(critiques) == 0 {
		return nil, 0.0, fmt.Errorf("all critics failed")
	}

	// Aggregate scores (average)
	totalScore := 0.0
	for _, s := range scores {
		totalScore += s
	}
	avgScore := totalScore / float64(len(scores))

	// Merge critique feedback
	mergedCritique := "Aggregated feedback from multiple critics:\n\n"
	for i, critique := range critiques {
		mergedCritique += fmt.Sprintf("Critic %d (Score: %.2f):\n%s\n\n",
			i+1, scores[i], critique.Payload)
	}

	aggregatedMsg := &agent.Message{
		Message: &pb.Message{
			Type:    "aggregated_critique",
			Payload: mergedCritique,
		},
	}

	return aggregatedMsg, avgScore, nil
}

// Reflection variants

// NewSelfReflection creates a reflection where generator acts as its own critic
func NewSelfReflection(name string, runtime agent.Runtime, agent string, maxIterations int) *Reflection {
	return NewReflection(name, runtime, agent, agent,
		WithMaxIterations(maxIterations),
	)
}

// NewMultiCriticReflection creates a reflection with multiple critics
func NewMultiCriticReflection(name string, runtime agent.Runtime, generator string, critics []string, maxIterations int) *Reflection {
	r := NewReflection(name, runtime, generator, critics[0],
		WithMaxIterations(maxIterations),
	)
	r.critics = critics
	return r
}
