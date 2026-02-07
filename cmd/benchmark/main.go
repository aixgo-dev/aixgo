package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/aixgo-dev/aixgo/internal/llm/evaluation"
	"github.com/aixgo-dev/aixgo/internal/llm/provider"
)

func main() {
	var (
		modelName    = flag.String("model", "gpt-4", "Model name to benchmark")
		providerName = flag.String("provider", "mock", "Provider name (mock, openai, etc)")
		outputFile   = flag.String("output", "", "Output file path (default: stdout)")
		outputFormat = flag.String("format", "text", "Output format: json, markdown, text")
		baseline     = flag.String("baseline", "", "Baseline JSON file for comparison")
		timeout      = flag.Duration("timeout", 5*time.Minute, "Overall benchmark timeout")
		suites       = flag.String("suites", "default", "Comma-separated test suite names")
		ciMode       = flag.Bool("ci", false, "CI mode: fail on regression")
	)
	flag.Parse()

	if err := run(*modelName, *providerName, *outputFile, *outputFormat, *baseline, *timeout, *suites, *ciMode); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(modelName, providerName, outputFile, outputFormat, baseline string, timeout time.Duration, suites string, ciMode bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Get provider
	p, err := getProvider(providerName, modelName)
	if err != nil {
		return fmt.Errorf("create provider: %w", err)
	}

	// Get test suites
	testSuites, err := getTestSuites(suites)
	if err != nil {
		return fmt.Errorf("get test suites: %w", err)
	}

	// Run benchmarks
	evaluator := evaluation.NewEvaluator(p, modelName)
	var benchmark *evaluation.ModelBenchmark

	for _, suite := range testSuites {
		result, err := evaluator.RunBenchmark(ctx, suite, modelName)
		if err != nil {
			return fmt.Errorf("run benchmark: %w", err)
		}
		if benchmark == nil {
			benchmark = result
		} else {
			// Merge results
			benchmark.Results = append(benchmark.Results, result.Results...)
		}
	}

	// Create report
	report := &evaluation.BenchmarkReport{
		Version:     "1.0.0",
		GeneratedAt: time.Now(),
		GitCommit:   getGitCommit(),
		GitBranch:   getGitBranch(),
		Environment: getEnvironment(),
		Benchmark:   benchmark,
	}

	// Compare with baseline if provided
	if baseline != "" {
		baselineReport, err := evaluation.LoadReport(baseline)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not load baseline: %v\n", err)
		} else {
			report.Comparison = evaluation.CompareWithBaseline(
				benchmark,
				baselineReport.Benchmark,
				baselineReport.GitCommit,
				baselineReport.GeneratedAt,
			)
		}
	}

	// Output
	format := evaluation.OutputFormat(outputFormat)
	var writer *os.File
	if outputFile != "" {
		f, err := os.Create(outputFile) // #nosec G304 - user-provided CLI argument
		if err != nil {
			return fmt.Errorf("create output file: %w", err)
		}
		defer func() {
			_ = f.Close()
		}()
		writer = f
	} else {
		writer = os.Stdout
	}

	if err := evaluation.FormatReport(report, format, writer); err != nil {
		return fmt.Errorf("format report: %w", err)
	}

	// Save JSON for CI artifacts (always save if output specified and not already json)
	if outputFile != "" && format != evaluation.FormatJSON {
		jsonPath := strings.TrimSuffix(outputFile, "."+string(format)) + ".json"
		if err := evaluation.SaveReport(report, jsonPath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not save JSON report: %v\n", err)
		}
	}

	// CI mode: fail on regression
	if ciMode && report.Comparison != nil && report.Comparison.HasRegression {
		return fmt.Errorf("benchmark regression detected")
	}

	return nil
}

func getProvider(name, model string) (provider.Provider, error) {
	switch name {
	case "mock":
		return provider.NewMockProvider(model), nil
	default:
		return nil, fmt.Errorf("unknown provider %q (only 'mock' supported in CLI)", name)
	}
}

func getTestSuites(names string) ([]*evaluation.TestSuite, error) {
	var suites []*evaluation.TestSuite
	for _, name := range strings.Split(names, ",") {
		name = strings.TrimSpace(name)
		switch name {
		case "default", "":
			suites = append(suites, evaluation.GetDefaultTestSuite())
		default:
			return nil, fmt.Errorf("unknown test suite: %s", name)
		}
	}
	if len(suites) == 0 {
		suites = append(suites, evaluation.GetDefaultTestSuite())
	}
	return suites, nil
}

func getGitCommit() string {
	if commit := os.Getenv("GITHUB_SHA"); commit != "" {
		return commit
	}
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func getGitBranch() string {
	if ref := os.Getenv("GITHUB_REF_NAME"); ref != "" {
		return ref
	}
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func getEnvironment() string {
	if os.Getenv("CI") != "" {
		return "ci"
	}
	return "local"
}
