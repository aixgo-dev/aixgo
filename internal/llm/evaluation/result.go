package evaluation

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"
)

// BenchmarkReport represents a complete benchmark report with metadata
type BenchmarkReport struct {
	Version     string           `json:"version"`
	GeneratedAt time.Time        `json:"generated_at"`
	GitCommit   string           `json:"git_commit,omitempty"`
	GitBranch   string           `json:"git_branch,omitempty"`
	Environment string           `json:"environment,omitempty"`
	Benchmark   *ModelBenchmark  `json:"benchmark"`
	Comparison  *BaselineCompare `json:"comparison,omitempty"`
}

// BaselineCompare represents comparison against a baseline
type BaselineCompare struct {
	BaselineCommit  string    `json:"baseline_commit"`
	BaselineDate    time.Time `json:"baseline_date"`
	SuccessRateDiff float64   `json:"success_rate_diff"`
	LatencyDiffMs   int64     `json:"latency_diff_ms"`
	TokensDiff      int       `json:"tokens_diff"`
	Regressions     []string  `json:"regressions,omitempty"`
	Improvements    []string  `json:"improvements,omitempty"`
	HasRegression   bool      `json:"has_regression"`
}

// SaveReport saves benchmark report to JSON file
func SaveReport(report *BenchmarkReport, path string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

// LoadReport loads benchmark report from JSON file
func LoadReport(path string) (*BenchmarkReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read report: %w", err)
	}
	var report BenchmarkReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("unmarshal report: %w", err)
	}
	return &report, nil
}

// CompareWithBaseline compares current benchmark with baseline
func CompareWithBaseline(current, baseline *ModelBenchmark, baselineCommit string, baselineDate time.Time) *BaselineCompare {
	compare := &BaselineCompare{
		BaselineCommit:  baselineCommit,
		BaselineDate:    baselineDate,
		SuccessRateDiff: current.Summary.SuccessRate - baseline.Summary.SuccessRate,
		LatencyDiffMs:   current.Summary.AverageLatency.Milliseconds() - baseline.Summary.AverageLatency.Milliseconds(),
		TokensDiff:      current.Summary.TotalTokens - baseline.Summary.TotalTokens,
	}

	// Check for regressions (success rate dropped by more than 5%)
	if compare.SuccessRateDiff < -0.05 {
		compare.Regressions = append(compare.Regressions,
			fmt.Sprintf("Success rate decreased by %.1f%%", -compare.SuccessRateDiff*100))
		compare.HasRegression = true
	}

	// Check for latency regressions (increased by more than 20%)
	if baseline.Summary.AverageLatency > 0 {
		latencyPctChange := float64(compare.LatencyDiffMs) / float64(baseline.Summary.AverageLatency.Milliseconds())
		if latencyPctChange > 0.20 {
			compare.Regressions = append(compare.Regressions,
				fmt.Sprintf("Average latency increased by %.1f%%", latencyPctChange*100))
			compare.HasRegression = true
		}
	}

	// Check for improvements
	if compare.SuccessRateDiff > 0.05 {
		compare.Improvements = append(compare.Improvements,
			fmt.Sprintf("Success rate improved by %.1f%%", compare.SuccessRateDiff*100))
	}

	return compare
}

// OutputFormat represents output format type
type OutputFormat string

const (
	FormatJSON     OutputFormat = "json"
	FormatMarkdown OutputFormat = "markdown"
	FormatText     OutputFormat = "text"
)

// FormatReport formats benchmark report in specified format
func FormatReport(report *BenchmarkReport, format OutputFormat, w io.Writer) error {
	switch format {
	case FormatJSON:
		return formatJSON(report, w)
	case FormatMarkdown:
		return formatMarkdown(report, w)
	case FormatText:
		return formatText(report, w)
	default:
		return fmt.Errorf("unknown format: %s", format)
	}
}

func formatJSON(report *BenchmarkReport, w io.Writer) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func formatMarkdown(report *BenchmarkReport, w io.Writer) error {
	var sb strings.Builder
	b := report.Benchmark

	sb.WriteString("# Benchmark Results\n\n")
	sb.WriteString(fmt.Sprintf("**Model:** %s  \n", b.ModelName))
	sb.WriteString(fmt.Sprintf("**Test Suite:** %s  \n", b.TestSuite))
	sb.WriteString(fmt.Sprintf("**Generated:** %s  \n", report.GeneratedAt.Format(time.RFC3339)))
	if report.GitCommit != "" {
		sb.WriteString(fmt.Sprintf("**Commit:** %s  \n", report.GitCommit))
	}
	sb.WriteString("\n")

	// Summary
	sb.WriteString("## Summary\n\n")
	sb.WriteString("| Metric | Value |\n")
	sb.WriteString("|--------|-------|\n")
	sb.WriteString(fmt.Sprintf("| Total Tests | %d |\n", b.Summary.TotalTests))
	sb.WriteString(fmt.Sprintf("| Passed | %d |\n", b.Summary.PassedTests))
	sb.WriteString(fmt.Sprintf("| Failed | %d |\n", b.Summary.FailedTests))
	sb.WriteString(fmt.Sprintf("| Success Rate | %.1f%% |\n", b.Summary.SuccessRate*100))
	sb.WriteString(fmt.Sprintf("| Avg Latency | %dms |\n", b.Summary.AverageLatency.Milliseconds()))
	sb.WriteString(fmt.Sprintf("| P95 Latency | %dms |\n", b.Summary.P95Latency.Milliseconds()))
	sb.WriteString(fmt.Sprintf("| Total Tokens | %d |\n", b.Summary.TotalTokens))
	sb.WriteString("\n")

	// Difficulty breakdown
	sb.WriteString("## By Difficulty\n\n")
	sb.WriteString("| Difficulty | Passed/Total | Rate |\n")
	sb.WriteString("|------------|--------------|------|\n")
	for _, diff := range []string{"easy", "medium", "hard"} {
		if stats, ok := b.Summary.DifficultyBreakdown[diff]; ok {
			sb.WriteString(fmt.Sprintf("| %s | %d/%d | %.1f%% |\n",
				diff, stats.Passed, stats.Total, stats.SuccessRate*100))
		}
	}
	sb.WriteString("\n")

	// Comparison if available
	if report.Comparison != nil {
		c := report.Comparison
		sb.WriteString("## Comparison with Baseline\n\n")
		sb.WriteString(fmt.Sprintf("**Baseline Commit:** %s  \n", c.BaselineCommit))
		sb.WriteString(fmt.Sprintf("**Success Rate Change:** %+.1f%%  \n", c.SuccessRateDiff*100))
		sb.WriteString(fmt.Sprintf("**Latency Change:** %+dms  \n", c.LatencyDiffMs))

		if len(c.Regressions) > 0 {
			sb.WriteString("\n### Regressions\n")
			for _, r := range c.Regressions {
				sb.WriteString(fmt.Sprintf("- %s\n", r))
			}
		}
		if len(c.Improvements) > 0 {
			sb.WriteString("\n### Improvements\n")
			for _, i := range c.Improvements {
				sb.WriteString(fmt.Sprintf("- %s\n", i))
			}
		}
	}

	_, err := w.Write([]byte(sb.String()))
	return err
}

func formatText(report *BenchmarkReport, w io.Writer) error {
	var sb strings.Builder
	b := report.Benchmark

	sb.WriteString("=== BENCHMARK RESULTS ===\n\n")
	sb.WriteString(fmt.Sprintf("Model:      %s\n", b.ModelName))
	sb.WriteString(fmt.Sprintf("Test Suite: %s\n", b.TestSuite))
	sb.WriteString(fmt.Sprintf("Generated:  %s\n", report.GeneratedAt.Format(time.RFC3339)))
	if report.GitCommit != "" {
		sb.WriteString(fmt.Sprintf("Commit:     %s\n", report.GitCommit))
	}
	sb.WriteString("\n--- Summary ---\n")
	sb.WriteString(fmt.Sprintf("Total:       %d\n", b.Summary.TotalTests))
	sb.WriteString(fmt.Sprintf("Passed:      %d\n", b.Summary.PassedTests))
	sb.WriteString(fmt.Sprintf("Failed:      %d\n", b.Summary.FailedTests))
	sb.WriteString(fmt.Sprintf("Success:     %.1f%%\n", b.Summary.SuccessRate*100))
	sb.WriteString(fmt.Sprintf("Avg Latency: %dms\n", b.Summary.AverageLatency.Milliseconds()))
	sb.WriteString(fmt.Sprintf("P95 Latency: %dms\n", b.Summary.P95Latency.Milliseconds()))

	sb.WriteString("\n--- By Difficulty ---\n")
	difficulties := make([]string, 0, len(b.Summary.DifficultyBreakdown))
	for d := range b.Summary.DifficultyBreakdown {
		difficulties = append(difficulties, d)
	}
	sort.Strings(difficulties)
	for _, diff := range difficulties {
		stats := b.Summary.DifficultyBreakdown[diff]
		sb.WriteString(fmt.Sprintf("%-8s %d/%d (%.1f%%)\n",
			diff+":", stats.Passed, stats.Total, stats.SuccessRate*100))
	}

	if report.Comparison != nil && report.Comparison.HasRegression {
		sb.WriteString("\n--- REGRESSIONS DETECTED ---\n")
		for _, r := range report.Comparison.Regressions {
			sb.WriteString(fmt.Sprintf("! %s\n", r))
		}
	}

	_, err := w.Write([]byte(sb.String()))
	return err
}
