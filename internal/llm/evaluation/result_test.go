package evaluation

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSaveAndLoadReport(t *testing.T) {
	report := &BenchmarkReport{
		Version:     "1.0.0",
		GeneratedAt: time.Now(),
		GitCommit:   "abc123",
		Benchmark: &ModelBenchmark{
			ModelName: "test-model",
			TestSuite: "test-suite",
			Summary: BenchmarkSummary{
				TotalTests:  10,
				PassedTests: 8,
				SuccessRate: 0.8,
			},
		},
	}

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "report.json")

	if err := SaveReport(report, path); err != nil {
		t.Fatalf("SaveReport failed: %v", err)
	}

	loaded, err := LoadReport(path)
	if err != nil {
		t.Fatalf("LoadReport failed: %v", err)
	}

	if loaded.GitCommit != report.GitCommit {
		t.Errorf("GitCommit = %q, want %q", loaded.GitCommit, report.GitCommit)
	}
	if loaded.Benchmark.Summary.SuccessRate != report.Benchmark.Summary.SuccessRate {
		t.Errorf("SuccessRate = %v, want %v", loaded.Benchmark.Summary.SuccessRate, report.Benchmark.Summary.SuccessRate)
	}
}

func TestLoadReportNotFound(t *testing.T) {
	_, err := LoadReport("/nonexistent/path.json")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestCompareWithBaseline(t *testing.T) {
	current := &ModelBenchmark{
		Summary: BenchmarkSummary{
			SuccessRate:    0.75,
			AverageLatency: 200 * time.Millisecond,
			TotalTokens:    1000,
		},
	}

	baseline := &ModelBenchmark{
		Summary: BenchmarkSummary{
			SuccessRate:    0.90,
			AverageLatency: 100 * time.Millisecond,
			TotalTokens:    800,
		},
	}

	compare := CompareWithBaseline(current, baseline, "base123", time.Now())

	if !compare.HasRegression {
		t.Error("expected regression to be detected")
	}
	if len(compare.Regressions) == 0 {
		t.Error("expected regression messages")
	}
	if compare.SuccessRateDiff >= 0 {
		t.Errorf("SuccessRateDiff = %v, expected negative", compare.SuccessRateDiff)
	}
}

func TestCompareWithBaseline_Improvement(t *testing.T) {
	current := &ModelBenchmark{
		Summary: BenchmarkSummary{
			SuccessRate:    0.95,
			AverageLatency: 100 * time.Millisecond,
			TotalTokens:    800,
		},
	}

	baseline := &ModelBenchmark{
		Summary: BenchmarkSummary{
			SuccessRate:    0.80,
			AverageLatency: 100 * time.Millisecond,
			TotalTokens:    800,
		},
	}

	compare := CompareWithBaseline(current, baseline, "base123", time.Now())

	if compare.HasRegression {
		t.Error("should not have regression")
	}
	if len(compare.Improvements) == 0 {
		t.Error("expected improvement messages")
	}
}

func TestFormatReport_JSON(t *testing.T) {
	report := createTestReport()
	var buf bytes.Buffer

	if err := FormatReport(report, FormatJSON, &buf); err != nil {
		t.Fatalf("FormatReport failed: %v", err)
	}

	if !strings.Contains(buf.String(), `"model_name"`) {
		t.Error("JSON output should contain model_name")
	}
}

func TestFormatReport_Markdown(t *testing.T) {
	report := createTestReport()
	var buf bytes.Buffer

	if err := FormatReport(report, FormatMarkdown, &buf); err != nil {
		t.Fatalf("FormatReport failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "# Benchmark Results") {
		t.Error("Markdown should contain header")
	}
	if !strings.Contains(output, "| Metric | Value |") {
		t.Error("Markdown should contain table")
	}
}

func TestFormatReport_Text(t *testing.T) {
	report := createTestReport()
	var buf bytes.Buffer

	if err := FormatReport(report, FormatText, &buf); err != nil {
		t.Fatalf("FormatReport failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "=== BENCHMARK RESULTS ===") {
		t.Error("Text should contain header")
	}
}

func TestFormatReport_Unknown(t *testing.T) {
	report := createTestReport()
	var buf bytes.Buffer

	err := FormatReport(report, "unknown", &buf)
	if err == nil {
		t.Error("expected error for unknown format")
	}
}

func TestFormatReport_WithComparison(t *testing.T) {
	report := createTestReport()
	report.Comparison = &BaselineCompare{
		BaselineCommit:  "abc123",
		SuccessRateDiff: -0.10,
		LatencyDiffMs:   50,
		Regressions:     []string{"Success rate decreased"},
		HasRegression:   true,
	}

	var buf bytes.Buffer
	if err := FormatReport(report, FormatMarkdown, &buf); err != nil {
		t.Fatalf("FormatReport failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Comparison with Baseline") {
		t.Error("Markdown should contain comparison section")
	}
	if !strings.Contains(output, "Regressions") {
		t.Error("Markdown should contain regressions section")
	}
}

func createTestReport() *BenchmarkReport {
	return &BenchmarkReport{
		Version:     "1.0.0",
		GeneratedAt: time.Now(),
		GitCommit:   "test123",
		Benchmark: &ModelBenchmark{
			ModelName: "test-model",
			TestSuite: "default",
			Summary: BenchmarkSummary{
				TotalTests:     10,
				PassedTests:    8,
				FailedTests:    2,
				SuccessRate:    0.8,
				AverageLatency: 150 * time.Millisecond,
				P95Latency:     300 * time.Millisecond,
				TotalTokens:    500,
				DifficultyBreakdown: map[string]*DifficultyStats{
					"easy":   {Total: 4, Passed: 4, SuccessRate: 1.0},
					"medium": {Total: 4, Passed: 3, SuccessRate: 0.75},
					"hard":   {Total: 2, Passed: 1, SuccessRate: 0.5},
				},
			},
		},
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
